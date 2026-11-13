package db_session

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	trexlogger "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
)

const defaultPostgresImage = "postgres:18"

// disableRyukIfPodman detects whether the container runtime is Podman and, if
// so, sets TESTCONTAINERS_RYUK_DISABLED=true before testcontainers reads its
// configuration. Ryuk hardcodes the Docker "bridge" network which does not
// exist in Podman, so it must be disabled when running against Podman.
//
// Explicit env-var values are never overwritten, so callers can always
// override auto-detection by setting TESTCONTAINERS_RYUK_DISABLED themselves.
func disableRyukIfPodman(ctx context.Context) {
	if os.Getenv("TESTCONTAINERS_RYUK_DISABLED") != "" {
		return
	}

	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer func() { _ = cli.Close() }()

	ver, err := cli.ServerVersion(ctx)
	if err != nil {
		return
	}

	for _, c := range ver.Components {
		if strings.Contains(c.Name, "Podman") {
			slog.InfoContext(ctx, "Detected Podman runtime, disabling Ryuk reaper (bridge network not available in Podman)")
			_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
			return
		}
	}
}

type Testcontainer struct {
	config    *config.DatabaseConfig
	container *postgres.PostgresContainer
	g2        *gorm.DB
	sqlDB     *sql.DB
}

var _ db.SessionFactory = &Testcontainer{}

// NewTestcontainerFactory creates a SessionFactory using testcontainers.
// This starts a real PostgreSQL container for integration testing.
func NewTestcontainerFactory(config *config.DatabaseConfig) *Testcontainer {
	conn := &Testcontainer{
		config: config,
	}
	conn.Init(config)
	return conn
}

func (f *Testcontainer) Init(config *config.DatabaseConfig) {
	ctx := context.Background()

	disableRyukIfPodman(ctx)

	image := os.Getenv("POSTGRES_IMAGE")
	if image == "" {
		image = defaultPostgresImage
	}

	slog.InfoContext(ctx, "Starting PostgreSQL testcontainer", "image", image)

	ctr, err := postgres.Run(ctx,
		image,
		postgres.WithDatabase(config.Name),
		postgres.WithUsername(config.Username),
		postgres.WithPassword(config.Password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				HostConfigModifier: func(hc *container.HostConfig) {
					hc.AutoRemove = true
				},
			},
		}),
	)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to start PostgreSQL testcontainer", "error", err)
		os.Exit(1)
	}

	f.container = ctr

	// Get connection string from container
	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get connection string from testcontainer", "error", err)
		os.Exit(1)
	}

	slog.InfoContext(ctx, "PostgreSQL testcontainer started", "connection_string", connStr)

	// Open SQL connection
	f.sqlDB, err = sql.Open("postgres", connStr)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to connect to testcontainer database", "error", err)
		os.Exit(1)
	}

	// Configure connection pool
	f.sqlDB.SetMaxOpenConns(config.MaxOpenConnections)

	// Connect GORM to use the same connection
	conf := &gorm.Config{
		PrepareStmt:            false,
		FullSaveAssociations:   false,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	}

	if config.Debug {
		conf.Logger = trexlogger.NewGormLogger().LogMode(logger.Info)
	}

	f.g2, err = gorm.Open(gormpostgres.New(gormpostgres.Config{
		Conn:                 f.sqlDB,
		PreferSimpleProtocol: true,
	}), conf)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to connect GORM to testcontainer database", "error", err)
		os.Exit(1)
	}

	// Run migrations
	slog.InfoContext(ctx, "Running database migrations on testcontainer")
	if err := db.Migrate(f.g2); err != nil {
		slog.ErrorContext(ctx, "Failed to run migrations on testcontainer", "error", err)
		os.Exit(1)
	}

	// Install cross-cutting GORM plugins after migrations so instrumentation
	// covers request traffic, not the one-off migration statements. With no
	// plugin registered this is a no-op.
	db.ApplyGormPlugins(f.g2)

	slog.InfoContext(ctx, "Testcontainer database initialized successfully")
}

func (f *Testcontainer) DirectDB() *sql.DB {
	return f.sqlDB
}

func (f *Testcontainer) New(ctx context.Context) *gorm.DB {
	conn := f.g2.Session(&gorm.Session{
		Context: ctx,
		Logger:  f.g2.Logger.LogMode(logger.Silent),
	})
	conn = db.BindTransaction(ctx, conn)
	if f.config.Debug {
		conn = conn.Debug()
	}
	return conn
}

func (f *Testcontainer) CheckConnection() error {
	_, err := f.sqlDB.Exec("SELECT 1")
	return err
}

func (f *Testcontainer) Close() error {
	ctx := context.Background()

	// Close SQL connection
	if f.sqlDB != nil {
		if err := f.sqlDB.Close(); err != nil {
			slog.ErrorContext(ctx, "Error closing SQL connection", "error", err)
		}
	}

	// Terminate container
	if f.container != nil {
		slog.InfoContext(ctx, "Stopping PostgreSQL testcontainer")
		if err := f.container.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate testcontainer: %s", err)
		}
		slog.InfoContext(ctx, "PostgreSQL testcontainer stopped")
	}

	return nil
}

func (f *Testcontainer) ResetDB() {
	ctx := context.Background()
	g2 := f.New(ctx)

	var tables []string
	err := g2.Raw(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		AND table_name != 'migrations'
	`).Scan(&tables).Error
	if err != nil {
		slog.ErrorContext(ctx, "Error querying tables", "error", err)
		return
	}

	for _, table := range tables {
		if err := g2.Exec(fmt.Sprintf("TRUNCATE TABLE \"%s\" CASCADE", table)).Error; err != nil {
			slog.ErrorContext(ctx, "Error truncating table", "table", table, "error", err)
		}
	}
}

func (f *Testcontainer) NewListener(ctx context.Context, channel string, callback func(id string)) {
	// Get the connection string for the listener
	connStr, err := f.container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get connection string for listener", "error", err)
		return
	}

	newListener(ctx, connStr, channel, callback)
}
