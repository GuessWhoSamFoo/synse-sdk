// Synse SDK
// Copyright (c) 2019 Vapor IO
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package sdk

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/patrickmn/go-cache"
	"github.com/rs/xid"
	"github.com/vapor-ware/synse-sdk/sdk/utils"
	synse "github.com/vapor-ware/synse-server-grpc/go"
)

const (
	statusPending = synse.WriteStatus_PENDING
	statusWriting = synse.WriteStatus_WRITING
	statusDone    = synse.WriteStatus_DONE
	statusError   = synse.WriteStatus_ERROR
)

// transactionCache is a cache with a configurable default expiration time that
// is used to track the asynchronous write transactions as they are processed.
var transactionCache *cache.Cache

// setupTransactionCache creates the transaction cache with the given TTL.
//
// This needs to be called prior to the plugin grpc server and device manager
// starting up in order for us to have transactions.
//
// Note that if this is called multiple times, the global transaction cache
// will be overwritten.
func setupTransactionCache(ttl time.Duration) {
	transactionCache = cache.New(ttl, ttl*2)
}

// newTransaction creates a new transaction instance. Upon creation, the
// transaction is given a unique ID and is added to the transaction cache.
//
// If the transaction cache has not been initialized by the time this is called,
// we will terminate the plugin, as it is indicative of an improper plugin setup.
func newTransaction(timeout time.Duration) *transaction {
	if transactionCache == nil {
		// FIXME - need to update log so we can specify our own exiter to test this..
		log.Fatalf("[sdk] transaction cache was not initialized; likely an issue in plugin setup")
	}

	id := xid.New().String()
	now := utils.GetCurrentTime()
	t := transaction{
		id:      id,
		status:  statusPending,
		created: now,
		updated: now,
		timeout: timeout,
		message: "",
	}
	transactionCache.Set(id, &t, cache.DefaultExpiration)
	return &t
}

// getTransaction looks up the given transaction ID in the cache. If it exists,
// that transaction is returned; otherwise nil is returned.
//
// If the transaction cache has not been initialized by the time this is called,
// we will terminate the plugin, as it is indicative of an improper plugin setup.
func getTransaction(id string) *transaction {
	if transactionCache == nil {
		log.Fatalf("[sdk] transaction cache was not initialized; likely an issue in plugin setup")
	}

	t, found := transactionCache.Get(id)
	if found {
		return t.(*transaction)
	}
	log.WithField("id", id).Info("[sdk] transaction not found")
	return nil
}

// transaction represents an asynchronous write transaction for the Plugin. It
// tracks the state and status of that transaction over its lifetime.
type transaction struct {
	id      string
	status  synse.WriteStatus
	created string
	updated string
	message string
	timeout time.Duration
}

// encode translates the transaction to a corresponding gRPC V3TransactionStatus.
func (t *transaction) encode() *synse.V3TransactionStatus {
	return &synse.V3TransactionStatus{
		Id:      t.id,
		Created: t.created,
		Updated: t.updated,
		Message: t.message,
		Timeout: t.timeout.String(),
		Status:  t.status,
	}
}

// setStatusPending sets the transaction status to 'pending'.
func (t *transaction) setStatusPending() {
	log.WithField("id", t.id).Debug("[transaction] transaction status set to PENDING")
	t.updated = utils.GetCurrentTime()
	t.status = statusPending
}

// setStatusWriting sets the transaction status to 'writing'.
func (t *transaction) setStatusWriting() {
	log.WithField("id", t.id).Debug("[transaction] transaction status set to WRITING")
	t.updated = utils.GetCurrentTime()
	t.status = statusWriting
}

// setStatusDone sets the transaction status to 'done'.
func (t *transaction) setStatusDone() {
	log.WithField("id", t.id).Debug("[transaction] transaction status set to DONE")
	t.updated = utils.GetCurrentTime()
	t.status = statusDone
}

// setStatusError sets the transaction status to 'error'.
func (t *transaction) setStatusError() {
	log.WithField("id", t.id).Debug("[transaction] transaction status set to ERROR")
	t.updated = utils.GetCurrentTime()
	t.status = statusError
}
