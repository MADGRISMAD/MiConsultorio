package api_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

// Cada prueba trabaja en su propia base, copiada de una plantilla que se migra una sola vez
// (CREATE DATABASE … TEMPLATE tarda ~100 ms; migrar desde cero, varios segundos). Así las pruebas no se pisan y
// pueden correr en paralelo. La plantilla y las copias se crean en el servidor de TEST_DATABASE_URL.

// serialTests tocan estado global del paquete y no pueden correr junto a otras:
// el reloj de citas (SetApptClock) y la llave de cifrado de expedientes (activeVault).
var serialTests = map[string]bool{
	"TestAgendaConflicts": true, "TestAgendaServiceAndFilters": true, "TestAgendaBlocks": true, "TestAgendaStatusTransitions": true,
	"TestAgendaEncounterLink": true, "TestAgendaSettings": true, "TestAgendaProfessionals": true,
	"TestEncFieldsAtRest": true, "TestEncFieldsOnlineBookingAndPortal": true, "TestEncFieldsLegacyRowsAndCommand": true,
	"TestEncFieldsReportsUnchanged": true, "TestEncFieldsValueMovedBetweenRows": true, "TestEncFieldsWrongKeyFailsControlled": true,
}

var (
	tmplOnce  sync.Once
	tmplName  string
	tmplErr   error
	adminPool *pgxpool.Pool
	createMu  sync.Mutex // CREATE DATABASE desde la misma plantilla, de uno en uno
	dbSeq     atomic.Int64
	parallel  sync.Map // *testing.T que ya llamaron t.Parallel (una prueba puede armar dos entornos)
)

func TestMain(m *testing.M) {
	code := m.Run()
	if adminPool != nil && tmplName != "" {
		_, _ = adminPool.Exec(context.Background(), `DROP DATABASE IF EXISTS `+tmplName+` WITH (FORCE)`)
		adminPool.Close()
	}
	os.Exit(code)
}

// withDB cambia el nombre de la base en una URL de Postgres.
func withDB(raw, name string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Path = "/" + name
	return u.String()
}

// testDB da a la prueba una base recién copiada de la plantilla migrada, y la borra al terminar.
func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	tmplOnce.Do(func() {
		if adminPool, tmplErr = pgxpool.New(ctx, base); tmplErr != nil {
			return
		}
		tmplName = fmt.Sprintf("caresia_tmpl_%d", os.Getpid())
		if _, tmplErr = adminPool.Exec(ctx, `DROP DATABASE IF EXISTS `+tmplName+` WITH (FORCE)`); tmplErr != nil {
			return
		}
		if _, tmplErr = adminPool.Exec(ctx, `CREATE DATABASE `+tmplName); tmplErr != nil {
			return
		}
		var p *pgxpool.Pool
		if p, tmplErr = db.Connect(ctx, withDB(base, tmplName)); tmplErr != nil {
			return
		}
		tmplErr = db.Migrate(ctx, p)
		p.Close() // una plantilla no puede tener conexiones abiertas al copiarse
	})
	if tmplErr != nil {
		t.Fatalf("plantilla de la base de pruebas: %v", tmplErr)
	}
	name := fmt.Sprintf("caresia_t_%d_%d", os.Getpid(), dbSeq.Add(1))
	createMu.Lock()
	_, err := adminPool.Exec(ctx, `CREATE DATABASE `+name+` TEMPLATE `+tmplName)
	createMu.Unlock()
	if err != nil {
		t.Fatalf("copiar la base de pruebas: %v", err)
	}
	pool, err := db.Connect(ctx, withDB(base, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`)
	})
	return pool
}

// maybeParallel pone la prueba en paralelo salvo que toque estado global (serialTests).
func maybeParallel(t *testing.T) {
	root := strings.SplitN(t.Name(), "/", 2)[0]
	if serialTests[root] {
		return
	}
	if _, done := parallel.LoadOrStore(t, true); !done {
		t.Parallel()
	}
}
