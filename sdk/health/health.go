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

package health

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vapor-ware/synse-server-grpc/go"
)

const (
	// TypePeriodic is the type name for periodic health checks.
	TypePeriodic = "periodic"
)

// DefaultCatalog is the default health check catalog that plugins can register
// health checks to.
var DefaultCatalog *Catalog

func init() {
	DefaultCatalog = NewCatalog()
}

// Catalog is the collection of all health checks registered with the plugin.
// Plugins should use the global catalog.
type Catalog struct {
	lock   sync.RWMutex
	checks map[string]Checker
}

// NewCatalog creates a new instance of a health check catalog.
func NewCatalog() *Catalog {
	return &Catalog{
		checks: make(map[string]Checker),
	}
}

// GetStatus gets the current health status from the catalog.
func (catalog *Catalog) GetStatus() []*Status {
	catalog.lock.RLock()
	defer catalog.lock.RUnlock()

	var statuses []*Status
	for name, check := range catalog.checks {
		s := check.Status()
		s.Name = name
		statuses = append(statuses, s)
	}
	return statuses
}

// GetStatus gets the current health status from the default health check catalog.
func GetStatus() []*Status {
	return DefaultCatalog.GetStatus()
}

// Register registers a checker with the catalog.
func (catalog *Catalog) Register(name string, check Checker) {
	catalog.lock.Lock()
	defer catalog.lock.Unlock()

	_, hasCheck := catalog.checks[name]
	if hasCheck {
		log.WithField("name", name).Fatalf("[health] health check already exists")
	}
	catalog.checks[name] = check
}

// Register registers a checker with the default health check catalog.
func Register(name string, check Checker) {
	DefaultCatalog.Register(name, check)
}

// RegisterPeriodicCheck registers a health check that will be run periodically.
func (catalog *Catalog) RegisterPeriodicCheck(name string, interval time.Duration, check Check) {
	log.WithFields(log.Fields{
		"name":     name,
		"interval": interval,
	}).Debug("[health] new periodic health check")
	catalog.Register(name, PeriodicChecker(check, interval))
}

// RegisterPeriodicCheck registers a health check that will be run periodically
// with the default health check catalog.
func RegisterPeriodicCheck(name string, interval time.Duration, check Check) {
	DefaultCatalog.RegisterPeriodicCheck(name, interval, check)
}

// Checker is the interface for a health checker. Anything that fulfils this
// interface should be able to provide health information about the plugin.
type Checker interface {
	Get() error
	Status() *Status
	Update(error)
}

// Check is a convenience type to define a function that can be used as a health check.
type Check func() error

// Status represents the status of a health checker at a given moment in time.
type Status struct {
	Name      string
	Ok        bool
	Message   string
	Timestamp string
	Type      string
}

// Encode converts the health Status into the Synse GRPC HealthCheck message.
func (status *Status) Encode() *synse.V3HealthCheck {

	// TODO (etd): could probably just encode this directly in the Status obj
	healthStatus := synse.HealthStatus_OK
	if !status.Ok {
		healthStatus = synse.HealthStatus_FAILING
	}

	return &synse.V3HealthCheck{
		Name:      status.Name,
		Status:    healthStatus,
		Message:   status.Message,
		Timestamp: status.Timestamp,
		Type:      status.Type,
	}
}

// checker implements the Checker interface and provides some basic functionality
// for the interface methods. The checker uses locking around its state, so it can
// be used safely asynchronously.
type checker struct {
	lock       sync.Mutex
	err        error
	lastUpdate string
	checkType  string
}

// NewChecker creates a new instance of a Checker.
func NewChecker(checkType string) Checker {
	return &checker{
		checkType: checkType,
	}
}

// Check implements the Checker interface. It gets the the Checker state in
// a non-blocking manner.
func (c *checker) Get() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.err
}

// Status implements the Checker interface. It gets the current Status of the
// Checker in a non-blocking manner.
func (c *checker) Status() *Status {
	var (
		message string
		ok      bool
	)

	err := c.Get()
	if err == nil {
		ok = true
		message = ""
	} else {
		ok = false
		message = err.Error()
	}
	return &Status{
		Name:      "",
		Ok:        ok,
		Message:   message,
		Timestamp: c.lastUpdate,
		Type:      c.checkType,
	}
}

// Update implements the Checker interface. It updates the Checker state in
// a non-blocking manner.
func (c *checker) Update(err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.lastUpdate = time.Now().Format(time.RFC3339)
	c.err = err
}

// PeriodicChecker creates a Checker for the given health check function and
// updates it periodically on the provided interval.
func PeriodicChecker(check Check, interval time.Duration) Checker {
	healthChecker := NewChecker(TypePeriodic)
	go func() {
		t := time.NewTicker(interval)
		for {
			<-t.C
			healthChecker.Update(check())
		}
	}()
	return healthChecker
}
