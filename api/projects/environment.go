package projects

import (
	"fmt"
	"net/http"

	"github.com/ansible-semaphore/semaphore/api/helpers"
	"github.com/ansible-semaphore/semaphore/db"

	"github.com/gorilla/context"
)

// EnvironmentMiddleware ensures an environment exists and loads it to the context
func EnvironmentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := context.Get(r, "project").(db.Project)
		envID, err := helpers.GetIntParam("environment_id", w, r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		env, err := helpers.Store(r).GetEnvironment(project.ID, envID)

		if err != nil {
			helpers.WriteError(w, err)
			return
		}

		context.Set(r, "environment", env)
		next.ServeHTTP(w, r)
	})
}

func GetEnvironmentRefs(w http.ResponseWriter, r *http.Request) {
	env := context.Get(r, "environment").(db.Environment)
	refs, err := helpers.Store(r).GetEnvironmentRefs(env.ProjectID, env.ID)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, refs)
}

// GetEnvironment retrieves sorted environments from the database
func GetEnvironment(w http.ResponseWriter, r *http.Request) {

	// return single environment if request has environment ID
	if environment := context.Get(r, "environment"); environment != nil {
		helpers.WriteJSON(w, http.StatusOK, environment.(db.Environment))
		return
	}

	project := context.Get(r, "project").(db.Project)

	env, err := helpers.Store(r).GetEnvironments(project.ID, helpers.QueryParams(r.URL))

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, env)
}

// UpdateEnvironment updates an existing environment in the database
func UpdateEnvironment(w http.ResponseWriter, r *http.Request) {
	oldEnv := context.Get(r, "environment").(db.Environment)
	var env db.Environment
	if !helpers.Bind(w, r, &env) {
		return
	}

	if env.ID != oldEnv.ID {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Environment ID in body and URL must be the same",
		})
		return
	}

	if env.ProjectID != oldEnv.ProjectID {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Project ID in body and URL must be the same",
		})
		return
	}

	if err := helpers.Store(r).UpdateEnvironment(env); err != nil {
		helpers.WriteError(w, err)
		return
	}

	helpers.EventLog(r, helpers.EventLogUpdate, helpers.EventLogItem{
		UserID:      helpers.UserFromContext(r).ID,
		ProjectID:   oldEnv.ProjectID,
		ObjectType:  db.EventEnvironment,
		ObjectID:    oldEnv.ID,
		Description: fmt.Sprintf("Environment %s updated", env.Name),
	})

	w.WriteHeader(http.StatusNoContent)
}

// AddEnvironment creates an environment in the database
func AddEnvironment(w http.ResponseWriter, r *http.Request) {
	project := context.Get(r, "project").(db.Project)
	var env db.Environment

	if !helpers.Bind(w, r, &env) {
		return
	}

	if project.ID != env.ProjectID {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Project ID in body and URL must be the same",
		})
	}

	newEnv, err := helpers.Store(r).CreateEnvironment(env)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	helpers.EventLog(r, helpers.EventLogCreate, helpers.EventLogItem{
		UserID:      helpers.UserFromContext(r).ID,
		ProjectID:   newEnv.ProjectID,
		ObjectType:  db.EventEnvironment,
		ObjectID:    newEnv.ID,
		Description: fmt.Sprintf("Environment %s created", newEnv.Name),
	})

	w.WriteHeader(http.StatusNoContent)
}

// RemoveEnvironment deletes an environment from the database
func RemoveEnvironment(w http.ResponseWriter, r *http.Request) {
	env := context.Get(r, "environment").(db.Environment)

	err := helpers.Store(r).DeleteEnvironment(env.ProjectID, env.ID)
	if err == db.ErrInvalidOperation {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "Environment is in use by one or more templates",
			"inUse": true,
		})
		return
	}

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	helpers.EventLog(r, helpers.EventLogDelete, helpers.EventLogItem{
		UserID:      helpers.UserFromContext(r).ID,
		ProjectID:   env.ProjectID,
		ObjectType:  db.EventEnvironment,
		ObjectID:    env.ID,
		Description: fmt.Sprintf("Environment %s deleted", env.Name),
	})

	w.WriteHeader(http.StatusNoContent)
}

type environmentSecret struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

func AddEnvironmentSecrets(w http.ResponseWriter, r *http.Request) {
	env := context.Get(r, "environment").(db.Environment)

	var secrets []environmentSecret

	if !helpers.Bind(w, r, &secrets) {
		return
	}

	store := helpers.Store(r)

	for _, secret := range secrets {
		_, err := store.CreateAccessKey(db.AccessKey{
			Name:          secret.Name,
			String:        secret.Secret,
			EnvironmentID: &env.ID,
			ProjectID:     &env.ProjectID,
			Type:          db.AccessKeyString,
		})

		if err != nil {
			continue
		}

	}

	w.WriteHeader(http.StatusNoContent)
}

func UpdateEnvironmentSecrets(w http.ResponseWriter, r *http.Request) {
	env := context.Get(r, "environment").(db.Environment)

	var secrets []environmentSecret

	if !helpers.Bind(w, r, &secrets) {
		return
	}

	store := helpers.Store(r)

	for _, secret := range secrets {
		key, err := store.GetAccessKey(env.ProjectID, secret.ID)

		if err != nil {
			continue
		}

		if key.EnvironmentID == nil && *key.EnvironmentID == env.ID {
			continue
		}

		err = store.UpdateAccessKey(db.AccessKey{
			Name:   secret.Name,
			String: secret.Secret,
			Type:   db.AccessKeyString,
		})

		if err != nil {
			continue
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func RemoveEnvironmentSecrets(w http.ResponseWriter, r *http.Request) {
	env := context.Get(r, "environment").(db.Environment)

	var secretIDs []int

	if !helpers.Bind(w, r, &secretIDs) {
		return
	}

	store := helpers.Store(r)

	for _, keyID := range secretIDs {
		key, err := store.GetAccessKey(env.ProjectID, keyID)

		if err != nil {
			continue
		}

		if key.EnvironmentID == nil && *key.EnvironmentID == env.ID {
			continue
		}

		err = store.DeleteAccessKey(env.ProjectID, keyID)

		if err != nil {
			continue
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
