package escrow

import (
	"fmt"
	"github.com/coreos/bbolt"
	"github.com/singnet/snet-daemon/blockchain"
	"github.com/singnet/snet-daemon/config"
	log "github.com/sirupsen/logrus"
	"math/big"
)

type combinedStorage struct {
	delegate PaymentChannelStorage
	mpe      *blockchain.MultiPartyEscrow
}

func NewCombinedStorage(processor *blockchain.Processor, delegate PaymentChannelStorage) PaymentChannelStorage {
	return &combinedStorage{
		delegate: delegate,
		mpe:      processor.MultiPartyEscrow(),
	}
}

func (storage *combinedStorage) Get(key *PaymentChannelKey) (state *PaymentChannelData, ok bool, err error) {
	log := log.WithField("key", key)

	// TODO: in fact we need to get latest actual state from storage by channel
	// id and if channel is not found then load its state from blockchain.
	// Then we should compare nonce with nonce which is sent by client, and
	// this logic can be moved into escrowPaymentHandler.
	state, ok, err = storage.delegate.Get(key)
	if ok && err == nil {
		return
	}
	if err != nil {
		return nil, false, err
	}
	log.Info("Channel key is not found in storage")

	state, ok, err = storage.getChannelStateFromBlockchain(key.ID)
	if !ok || err != nil {
		return
	}
	log = log.WithField("state", state)
	log.Info("Channel found in blockchain")

	// TODO: see comment at the beginning of the method
	if state.Nonce.Cmp(key.Nonce) != 0 {
		log.Warn("Channel nonce is not equal to expected")
		return nil, false, fmt.Errorf("Channel nonce: %v is not equal to expected: %v", state.Nonce, key.Nonce)
	}

	ok, err = storage.CompareAndSwap(key, nil, state)
	if err != nil {
		return
	}
	if !ok {
		log.Warn("Key is already present in the storage")
		return nil, false, err
	}
	log.WithField("state", state).Info("Channel saved in storage")

	return
}

func (storage *combinedStorage) getChannelStateFromBlockchain(id *big.Int) (state *PaymentChannelData, ok bool, err error) {
	log := log.WithField("id", id)

	channel, err := storage.mpe.Channels(nil, id)
	if err != nil {
		log.WithError(err).Warn("Unable to find channel id in blockchain")
		return nil, false, err
	}
	log = log.WithField("channel", channel)
	log.Debug("Channel found in blockchain")

	configGroupId := config.GetBigInt(config.ReplicaGroupIDKey)
	if channel.ReplicaId.Cmp(configGroupId) != 0 {
		log.WithField("configGroupId", configGroupId).Warn("Channel received belongs to another group of replicas")
		return nil, false, fmt.Errorf("Channel received belongs to another group of replicas, current group: %v, channel group: %v", configGroupId, channel.ReplicaId)
	}

	return &PaymentChannelData{
		Nonce:      channel.Nonce,
		State:      Open,
		Sender:     channel.Sender,
		Recipient:  channel.Recipient,
		GroupId:    channel.ReplicaId,
		FullAmount: channel.Value,
		//Expiration:       channel.Expiration,
		AuthorizedAmount: big.NewInt(0),
		Signature:        nil,
	}, true, nil
}

func (storage *combinedStorage) Put(key *PaymentChannelKey, state *PaymentChannelData) (err error) {
	return storage.delegate.Put(key, state)
}

func (storage *combinedStorage) CompareAndSwap(key *PaymentChannelKey, prevState *PaymentChannelData, newState *PaymentChannelData) (ok bool, err error) {
	return storage.delegate.CompareAndSwap(key, prevState, newState)
}

func NewDbStorage(db *bolt.DB) (storage PaymentChannelStorage) {
	// TODO: implement
	return nil
}
