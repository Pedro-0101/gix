package db

import (
	"fmt"
	"strings"
	"time"
)

// Alert is a reminder that fires at FireAt. NoteID is an optional weak link to
// the originating note. Recurrence is "" for one-shot, or the closed JSON rule
// (see internal/app/recurrence.go) for repeating alerts. Status is
// pending|done|cancelled. FireAt is stored in UTC.
type Alert struct {
	ID         int64
	Message    string
	NoteID     *int64
	FireAt     time.Time
	Recurrence string
	Status     string
	CreatedAt  string
}

const alertTimeLayout = time.RFC3339

const alertCols = "id, message, note_id, fire_at, recurrence, status, created_at"

// CreateAlert inserts a reminder and returns its id. FireAt is stored in UTC.
func (d *Database) CreateAlert(a Alert) (int64, error) {
	res, err := d.db.Exec(
		"INSERT INTO alerts (message, note_id, fire_at, recurrence) VALUES (?, ?, ?, ?)",
		a.Message, a.NoteID, a.FireAt.UTC().Format(alertTimeLayout), a.Recurrence)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// scanAlert reads one alert row in the canonical column order, parsing fire_at.
func scanAlert(s interface{ Scan(...any) error }) (Alert, error) {
	var a Alert
	var fireAt string
	if err := s.Scan(&a.ID, &a.Message, &a.NoteID, &fireAt, &a.Recurrence, &a.Status, &a.CreatedAt); err != nil {
		return Alert{}, err
	}
	t, err := time.Parse(alertTimeLayout, fireAt)
	if err != nil {
		return Alert{}, fmt.Errorf("scanAlert: fire_at %q: %w", fireAt, err)
	}
	a.FireAt = t
	return a, nil
}

// GetAlert returns one alert by id.
func (d *Database) GetAlert(id int64) (Alert, error) {
	return scanAlert(d.db.QueryRow("SELECT "+alertCols+" FROM alerts WHERE id = ?", id))
}

func (d *Database) queryAlerts(query string, args ...any) ([]Alert, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alert
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListAlerts returns alerts filtered by status (empty = all), fire_at ascending.
func (d *Database) ListAlerts(statuses ...string) ([]Alert, error) {
	if len(statuses) == 0 {
		return d.queryAlerts("SELECT " + alertCols + " FROM alerts ORDER BY fire_at ASC, id ASC")
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(statuses)), ",")
	args := make([]any, len(statuses))
	for i, s := range statuses {
		args[i] = s
	}
	return d.queryAlerts(
		"SELECT "+alertCols+" FROM alerts WHERE status IN ("+placeholders+") ORDER BY fire_at ASC, id ASC", args...)
}

// DueAlerts returns pending alerts whose fire_at has passed, fire_at ascending.
func (d *Database) DueAlerts(now time.Time) ([]Alert, error) {
	return d.queryAlerts(
		"SELECT "+alertCols+" FROM alerts WHERE status = 'pending' AND fire_at <= ? ORDER BY fire_at ASC, id ASC",
		now.UTC().Format(alertTimeLayout))
}

// UpdateAlertFireAt reschedules an alert (used by recurrence and snooze). The
// alert stays pending.
func (d *Database) UpdateAlertFireAt(id int64, fireAt time.Time) error {
	_, err := d.db.Exec("UPDATE alerts SET fire_at = ? WHERE id = ?", fireAt.UTC().Format(alertTimeLayout), id)
	return err
}

// SetAlertStatus sets an alert's status (pending|done|cancelled).
func (d *Database) SetAlertStatus(id int64, status string) error {
	_, err := d.db.Exec("UPDATE alerts SET status = ? WHERE id = ?", status, id)
	return err
}

// DeleteAlert physically removes an alert.
func (d *Database) DeleteAlert(id int64) error {
	_, err := d.db.Exec("DELETE FROM alerts WHERE id = ?", id)
	return err
}
