package db

import "time"

// Event represents information generated by ansible or api action captured to the database during execution
type Event struct {
	ProjectID   *int      `db:"project_id" json:"project_id"`
	ObjectID    *int      `db:"object_id" json:"object_id"`
	ObjectType  *string   `db:"object_type" json:"object_type"`
	Description *string   `db:"description" json:"description"`
	Created     time.Time `db:"created" json:"created"`

	ObjectName  string  `db:"-" json:"object_name"`
	ProjectName *string `db:"project_name" json:"project_name"`
}

// Insert writes the event to the database
func (evt Event) Insert() error {
	_, err := Sql.Exec(
		"insert into event(project_id, object_id, object_type, description, created) values (?, ?, ?, ?, ?)",
		evt.ProjectID,
		evt.ObjectID,
		evt.ObjectType,
		evt.Description,
		time.Now().Format("2006-01-02 15:04:05"))

	return err
}
