package db_session

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/lib/pq"

	"github.com/openshift-online/rh-trex-ai/pkg/config"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	trexlogger "github.com/openshift-online/rh-trex-ai/pkg/logger"
)

type Default struct {
	config *config.DatabaseConfig

	g2 *gorm.DB
	// Direct database connection.
	// It is used:
	// - to setup/close connection because GORM V2 removed gorm.Close()
	// - to work with pq.CopyIn because connection returned by GORM V2 gorm.DB() in "not the same"
	db *sql.DB
}

var _ db.SessionFactory = &Default{}

func NewProdFactory(config *config.DatabaseConfig) *Default {
	conn := &Default{}
	conn.Init(config)
	return conn
}

// Init will initialize a singleton connection as needed and return the same instance.
// Go includes database connection pooling in the platform. Gorm uses the same and provides a method to
// clone a connection via New(), which is safe for use by concurrent Goroutines.
func (f *Default) Init(config *config.DatabaseConfig) {
	// Only the first time
	once.Do(func() {
		var (
			g2  *gorm.DB
			err error
		)

		dsn := config.ConnectionString(config.SSLMode != disable)

		conf := &gorm.Config{
			PrepareStmt:          false,
			FullSaveAssociations: false,
		}
		g2, err = gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		}), conf)
		if err != nil {
			dsn = config.ConnectionString(false)
			g2, err = gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true,
			}), conf)
			if err != nil {
				panic(fmt.Sprintf(
					"GORM failed to connect to %s database %s with connection string: %s\nError: %s",
					config.Dialect,
					config.Name,
					config.LogSafeConnectionString(config.SSLMode != disable),
					err.Error(),
				))
			}
		}

		dbx, err := g2.DB()
		if err != nil {
			panic(fmt.Sprintf(
				"failed to get underlying *sql.DB: %s", err.Error(),
			))
		}
		dbx.SetMaxOpenConns(config.MaxOpenConnections)

		f.config = config
		f.g2 = g2
		f.db = dbx
	})
}

func (f *Default) DirectDB() *sql.DB {
	return f.db
}

func waitForNotification(ctx context.Context, l *pq.Listener, callback func(id string)) bool {
	logger := trexlogger.NewLogger(ctx)
	select {
	case <-ctx.Done():
		return false
	case n := <-l.Notify:
		if n != nil {
			logger.Infof("Received data from channel [%s] : %s", n.Channel, n.Extra)
			callback(n.Extra)
		}
		return true
	case <-time.After(10 * time.Second):
		go func() {
			if err := l.Ping(); err != nil {
				logger.V(5).Infof("Listener ping error: %v", err)
			}
		}()
		return true
	}
}

func newListener(ctx context.Context, connstr, channel string, callback func(id string)) {
	logger := trexlogger.NewLogger(ctx)

	plog := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			logger.Error(err.Error())
		}
	}
	listener := pq.NewListener(connstr, 10*time.Second, time.Minute, plog)
	err := listener.Listen(channel)
	if err != nil {
		panic(err)
	}

	logger.Infof("Starting channeling monitor for %s", channel)
	for waitForNotification(ctx, listener, callback) {
	}

	if err := listener.Close(); err != nil {
		logger.V(5).Infof("Error closing listener: %v", err)
	}
	logger.Infof("Stopped channeling monitor for %s", channel)
}

func (f *Default) NewListener(ctx context.Context, channel string, callback func(id string)) {
	newListener(ctx, f.config.ConnectionString(true), channel, callback)
}

func (f *Default) New(ctx context.Context) *gorm.DB {
	conn := f.g2.Session(&gorm.Session{
		Context: ctx,
		Logger:  f.g2.Logger.LogMode(logger.Silent),
	})
	if f.config.Debug {
		conn = conn.Debug()
	}
	return conn
}

func (f *Default) CheckConnection() error {
	return f.g2.Exec("SELECT 1").Error
}

// Close will close the connection to the database.
// THIS MUST **NOT** BE CALLED UNTIL THE SERVER/PROCESS IS EXITING!!
// This should only ever be called once for the entire duration of the application and only at the end.
func (f *Default) Close() error {
	return f.db.Close()
}

func (f *Default) ResetDB() {
	panic("ResetDB is not implemented for non-integration-test env")
}
