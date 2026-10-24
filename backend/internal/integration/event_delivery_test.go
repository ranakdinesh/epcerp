package integration

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	eventOut "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventApp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	eventDomain "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/domain"
	"sync"
	"testing"
	"time"
)

func eventFixture() eventDomain.Event {
	id := eventOut.SystemValues{}.ID()
	return eventDomain.Event{ID: id, Module: "fixture", EntityType: "source", EntityID: eventOut.SystemValues{}.ID(), EntityVersion: 1, ActorKind: "system", Action: "created", CorrelationID: id, ScopeKind: "platform", Visibility: "restricted", OccurredAt: time.Now().UTC().Truncate(time.Microsecond)}
}
func publish(t *testing.T, pool *pgxpool.Pool, event eventDomain.Event, targets ...eventDomain.Target) error {
	t.Helper()
	return database.NewManager(pool).Within(context.Background(), func(ctx context.Context, c database.Connection) error {
		return eventApp.NewRecorder(eventOut.NewJournal(c)).Record(ctx, event, targets)
	})
}
func countRows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
func execFixture(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}
func TestEventJournalAtomicityAndReplay(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	e := eventFixture()
	if err := eventApp.NewRecorder(eventOut.NewJournal(pool)).Record(ctx, e, nil); !errors.Is(err, eventDomain.ErrTransactionRequired) {
		t.Fatal("pool publication accepted", err)
	}
	target := eventDomain.Target{Consumer: "fixture", MaxAttempts: 3}
	for i := 0; i < 2; i++ {
		if err := publish(t, pool, e, target); err != nil {
			t.Fatal("replay rejected", err)
		}
	}
	if countRows(t, pool, "business_events") != 1 || countRows(t, pool, "event_deliveries") != 1 {
		t.Fatal("replay duplicated journal/delivery")
	}
	if err := publish(t, pool, e); !errors.Is(err, eventDomain.ErrConflict) {
		t.Fatal("removed target accepted", err)
	}
	if err := publish(t, pool, e, eventDomain.Target{Consumer: "changed_consumer", MaxAttempts: 3}); !errors.Is(err, eventDomain.ErrConflict) {
		t.Fatal("changed routing accepted", err)
	}
	conflict := e
	conflict.Action = "changed"
	if err := publish(t, pool, conflict, target); !errors.Is(err, eventDomain.ErrConflict) {
		t.Fatal("altered replay accepted", err)
	}
	if err := publish(t, pool, e, eventDomain.Target{Consumer: "fixture", MaxAttempts: 4}); !errors.Is(err, eventDomain.ErrConflict) {
		t.Fatal("changed retry policy accepted", err)
	}
	conflict = e
	conflict.ID = eventOut.SystemValues{}.ID()
	if err := publish(t, pool, conflict, target); !errors.Is(err, eventDomain.ErrConflict) {
		t.Fatal("duplicate source version accepted", err)
	}
	rolled := eventFixture()
	cause := errors.New("source command failed")
	err := database.NewManager(pool).Within(ctx, func(ctx context.Context, c database.Connection) error {
		if _, err := c.Exec(ctx, "INSERT INTO industries(id,name,normalized_name,code,is_active) VALUES($1,'Source fixture','source fixture','SOURCE',true)", rolled.EntityID); err != nil {
			return err
		}
		if err := eventApp.NewRecorder(eventOut.NewJournal(c)).Record(ctx, rolled, []eventDomain.Target{target}); err != nil {
			return err
		}
		return cause
	})
	if !errors.Is(err, cause) || countRows(t, pool, "industries") != 0 || countRows(t, pool, "business_events") != 1 || countRows(t, pool, "event_deliveries") != 1 {
		t.Fatal("source failure leaked event/delivery", err)
	}
	// Force a real deferred commit error after the recorder and source write succeeded.
	execFixture(t, pool, `CREATE FUNCTION fixture_commit_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture deferred failure' USING ERRCODE='23514'; END; $$;
 CREATE CONSTRAINT TRIGGER fixture_commit_failure AFTER INSERT ON event_deliveries DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.consumer='commit_failure') EXECUTE FUNCTION fixture_commit_failure();`)
	failed := eventFixture()
	err = database.NewManager(pool).Within(ctx, func(ctx context.Context, c database.Connection) error {
		if _, err := c.Exec(ctx, "INSERT INTO industries(id,name,normalized_name,code,is_active) VALUES($1,'Commit fixture','commit fixture','COMMIT',true)", failed.EntityID); err != nil {
			return err
		}
		return eventApp.NewRecorder(eventOut.NewJournal(c)).Record(ctx, failed, []eventDomain.Target{{Consumer: "commit_failure", MaxAttempts: 2}})
	})
	if err == nil || countRows(t, pool, "industries") != 0 || countRows(t, pool, "business_events") != 1 || countRows(t, pool, "event_deliveries") != 1 {
		t.Fatal("commit failure leaked source/event/delivery")
	}
	execFixture(t, pool, "DROP TRIGGER fixture_commit_failure ON event_deliveries; DROP FUNCTION fixture_commit_failure()")
	for _, sql := range []string{"UPDATE business_events SET action='changed'", "DELETE FROM business_events", "TRUNCATE business_events"} {
		if _, err := pool.Exec(ctx, sql); err == nil {
			t.Fatal("immutable events changed", sql)
		}
	}
}
func TestEventQueueLeaseAndRetry(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	e := eventFixture()
	if err := publish(t, pool, e, eventDomain.Target{Consumer: "fixture", MaxAttempts: 3}); err != nil {
		t.Fatal(err)
	}
	q := eventOut.NewQueue(pool)
	first, err := q.Claim(ctx, "fixture", eventOut.SystemValues{}.ID(), 30)
	if err != nil || first == nil || first.Attempt != 1 {
		t.Fatal("first claim", err)
	}
	if other, err := q.Claim(ctx, "fixture", eventOut.SystemValues{}.ID(), 30); err != nil || other != nil {
		t.Fatal("live lease claimed twice", err)
	}
	wrong := *first
	wrong.LeaseToken = eventOut.SystemValues{}.ID()
	if err := q.Complete(ctx, wrong); !errors.Is(err, eventDomain.ErrLeaseLost) {
		t.Fatal("wrong worker acknowledged", err)
	}
	execFixture(t, pool, "UPDATE event_deliveries SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1", first.ID)
	if err := q.Complete(ctx, *first); !errors.Is(err, eventDomain.ErrLeaseLost) {
		t.Fatal("expired lease acknowledged", err)
	}
	second, err := q.Claim(ctx, "fixture", eventOut.SystemValues{}.ID(), 30)
	if err != nil || second == nil || second.Attempt != 2 || second.LeaseToken == first.LeaseToken {
		t.Fatal("crash recovery", err)
	}
	if err := q.Fail(ctx, *first, 1); !errors.Is(err, eventDomain.ErrLeaseLost) {
		t.Fatal("stale worker changed replacement lease", err)
	}
	if err := q.Fail(ctx, *second, 1); err != nil {
		t.Fatal(err)
	}
	if claim, err := q.Claim(ctx, "fixture", eventOut.SystemValues{}.ID(), 30); err != nil || claim != nil {
		t.Fatal("retry backoff ignored", err)
	}
	execFixture(t, pool, "UPDATE event_deliveries SET available_at=clock_timestamp()-interval '1 second' WHERE id=$1", first.ID)
	third, err := q.Claim(ctx, "fixture", eventOut.SystemValues{}.ID(), 30)
	if err != nil || third == nil || third.Attempt != 3 {
		t.Fatal("last attempt", err)
	}
	if err := q.Fail(ctx, *third, 1); err != nil {
		t.Fatal(err)
	}
	var state string
	var attempts int
	if err := pool.QueryRow(ctx, "SELECT state,attempts FROM event_deliveries WHERE id=$1", first.ID).Scan(&state, &attempts); err != nil || state != "dead" || attempts != 3 {
		t.Fatal("exhausted delivery not retained", state, attempts, err)
	}
	if claim, err := q.Claim(ctx, "fixture", eventOut.SystemValues{}.ID(), 30); err != nil || claim != nil {
		t.Fatal("dead delivery reclaimed", err)
	}
	if countRows(t, pool, "event_delivery_attempts") != 6 {
		t.Fatal("missing claimed/expired/retry/dead evidence")
	}
	for _, sql := range []string{"DELETE FROM event_delivery_attempts", "UPDATE event_delivery_attempts SET kind='delivered'", "TRUNCATE event_delivery_attempts"} {
		if _, err := pool.Exec(ctx, sql); err == nil {
			t.Fatal("attempt history altered")
		}
	}
}
func TestEventConcurrentClaimsAndExhaustedCrash(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	for i := 0; i < 12; i++ {
		if err := publish(t, pool, eventFixture(), eventDomain.Target{Consumer: "parallel", MaxAttempts: 1}); err != nil {
			t.Fatal(err)
		}
	}
	q := eventOut.NewQueue(pool)
	var group sync.WaitGroup
	claims := make(chan *eventDomain.Claim, 12)
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			claim, err := q.Claim(ctx, "parallel", eventOut.SystemValues{}.ID(), 30)
			if err != nil {
				errs <- err
				return
			}
			claims <- claim
		}()
	}
	group.Wait()
	close(claims)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var last *eventDomain.Claim
	for claim := range claims {
		if claim == nil || seen[claim.ID] {
			t.Fatal("duplicate/missing concurrent claim")
		}
		seen[claim.ID] = true
		last = claim
	}
	if len(seen) != 12 {
		t.Fatal("missing concurrent deliveries")
	}
	if err := q.Complete(ctx, *last); err != nil {
		t.Fatal(err)
	}
	if err := q.Complete(ctx, *last); !errors.Is(err, eventDomain.ErrLeaseLost) {
		t.Fatal("duplicate acknowledgement changed history", err)
	}
	execFixture(t, pool, "UPDATE event_deliveries SET lease_until=clock_timestamp()-interval '1 second' WHERE state='leased'")
	if n, err := q.ExpireExhausted(ctx, "parallel"); err != nil || n != 11 {
		t.Fatal("last-attempt crash stuck", n, err)
	}
	if n, err := q.ExpireExhausted(ctx, "parallel"); err != nil || n != 0 {
		t.Fatal("expiry duplicated", n, err)
	}
	if claim, err := q.Claim(ctx, "parallel", eventOut.SystemValues{}.ID(), 30); err != nil || claim != nil {
		t.Fatal("terminal state reclaimed", err)
	}
	if countRows(t, pool, "event_delivery_attempts") != 24 {
		t.Fatal("missing concurrent transition evidence")
	}
}
