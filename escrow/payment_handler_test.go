package escrow

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/singnet/snet-daemon/blockchain"
	"github.com/singnet/snet-daemon/handler"
)

type PaymentHandlerTestSuite struct {
	suite.Suite

	paymentChannelServiceMock PaymentChannelService
	incomeValidatorMock       IncomeValidator

	paymentHandler paymentChannelPaymentHandler
}

func (suite *PaymentHandlerTestSuite) SetupSuite() {
	suite.paymentChannelServiceMock = &paymentChannelServiceMock{
		data: suite.channel(),
		err:  nil,
	}
	suite.incomeValidatorMock = &incomeValidatorMockType{}

	suite.paymentHandler = paymentChannelPaymentHandler{
		service:            suite.paymentChannelServiceMock,
		mpeContractAddress: func() common.Address { return blockchain.HexToAddress("0xf25186b5081ff5ce73482ad761db0eb0d25abfbf") },
		incomeValidator:    suite.incomeValidatorMock,
	}
}

func TestPaymentHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(PaymentHandlerTestSuite))
}

func (suite *PaymentHandlerTestSuite) channel() *PaymentChannelData {
	return &PaymentChannelData{
		AuthorizedAmount: big.NewInt(12300),
	}
}

func (suite *PaymentHandlerTestSuite) grpcMetadata(channelID, channelNonce, amount int64, signature []byte) metadata.MD {
	md := metadata.New(map[string]string{})

	md.Set(PaymentChannelIDHeader, strconv.FormatInt(channelID, 10))
	md.Set(PaymentChannelNonceHeader, strconv.FormatInt(channelNonce, 10))
	md.Set(PaymentChannelAmountHeader, strconv.FormatInt(amount, 10))
	md.Set(PaymentChannelSignatureHeader, string(signature))

	return md
}

func (suite *PaymentHandlerTestSuite) grpcContext(patch func(*metadata.MD)) *handler.GrpcStreamContext {
	md := suite.grpcMetadata(42, 3, 12345, []byte{0x1, 0x2, 0xFE, 0xFF})
	patch(&md)
	return &handler.GrpcStreamContext{
		MD: md,
	}
}

func (suite *PaymentHandlerTestSuite) TestGetPayment() {
	context := suite.grpcContext(func(md *metadata.MD) {})

	_, err := suite.paymentHandler.Payment(context)

	assert.Nil(suite.T(), err, "Unexpected error: %v", err)
}

func (suite *PaymentHandlerTestSuite) TestGetPaymentNoChannelId() {
	context := suite.grpcContext(func(md *metadata.MD) {
		delete(*md, PaymentChannelIDHeader)
	})

	payment, err := suite.paymentHandler.Payment(context)

	assert.Equal(suite.T(), handler.NewGrpcError(codes.InvalidArgument, "missing \"snet-payment-channel-id\""), err)
	assert.Nil(suite.T(), payment)
}

func (suite *PaymentHandlerTestSuite) TestGetPaymentNoChannelNonce() {
	context := suite.grpcContext(func(md *metadata.MD) {
		delete(*md, PaymentChannelNonceHeader)
	})

	payment, err := suite.paymentHandler.Payment(context)

	assert.Equal(suite.T(), handler.NewGrpcError(codes.InvalidArgument, "missing \"snet-payment-channel-nonce\""), err)
	assert.Nil(suite.T(), payment)
}

func (suite *PaymentHandlerTestSuite) TestGetPaymentNoChannelAmount() {
	context := suite.grpcContext(func(md *metadata.MD) {
		delete(*md, PaymentChannelAmountHeader)
	})

	payment, err := suite.paymentHandler.Payment(context)

	assert.Equal(suite.T(), handler.NewGrpcError(codes.InvalidArgument, "missing \"snet-payment-channel-amount\""), err)
	assert.Nil(suite.T(), payment)
}

func (suite *PaymentHandlerTestSuite) TestGetPaymentNoSignature() {
	context := suite.grpcContext(func(md *metadata.MD) {
		delete(*md, PaymentChannelSignatureHeader)
	})

	payment, err := suite.paymentHandler.Payment(context)

	assert.Equal(suite.T(), handler.NewGrpcError(codes.InvalidArgument, "missing \"snet-payment-channel-signature-bin\""), err)
	assert.Nil(suite.T(), payment)
}

func (suite *PaymentHandlerTestSuite) TestStartTransactionError() {
	context := suite.grpcContext(func(md *metadata.MD) {})
	paymentHandler := suite.paymentHandler
	paymentHandler.service = &paymentChannelServiceMock{
		err: NewPaymentError(FailedPrecondition, "another transaction in progress"),
	}

	payment, err := paymentHandler.Payment(context)

	assert.Equal(suite.T(), handler.NewGrpcError(codes.FailedPrecondition, "another transaction in progress"), err)
	assert.Nil(suite.T(), payment)
}

func (suite *PaymentHandlerTestSuite) TestValidatePaymentIncorrectIncome() {
	context := suite.grpcContext(func(md *metadata.MD) {})
	incomeErr := NewPaymentError(Unauthenticated, "incorrect payment income: \"45\", expected \"46\"")
	paymentHandler := suite.paymentHandler
	paymentHandler.incomeValidator = &incomeValidatorMockType{err: incomeErr}

	payment, err := paymentHandler.Payment(context)

	assert.Equal(suite.T(), handler.NewGrpcError(codes.Unauthenticated, "incorrect payment income: \"45\", expected \"46\""), err)
	assert.Nil(suite.T(), payment)
}
