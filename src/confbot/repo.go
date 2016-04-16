package confbot

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/mediocregopher/radix.v2/pool"
)

const (
	baseNamespace = "confbot"
	keySeperator  = ":"
)

// ProjectExistsErr is a project exists error.
type ProjectExistsErr struct{}

var _ error = (*ProjectExistsErr)(nil)

func (e ProjectExistsErr) Error() string {
	return "project exists"
}

// Repo is a repository for managing confbot data
type Repo interface {
	RegisterProject(id, userID string) error
	ProjectID(userID string) (string, error)
	User(projectID string) (string, error)
}

// NewRepo creates an instance of Repo. Repo is currently
// backed with a redis implementation.
func NewRepo(ctx context.Context, redisURL, env string) (Repo, error) {
	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, err
	}

	addr := u.Host

	p, err := pool.New("tcp", addr, 10)
	if err != nil {
		return nil, err
	}

	repo := &redisRepo{
		pool:      p,
		namespace: strings.Join([]string{baseNamespace, env}, keySeperator),
		log:       logFromContext(ctx),
	}

	return repo, nil
}

type redisRepo struct {
	pool      *pool.Pool
	namespace string
	log       *logrus.Entry
}

var _ Repo = (*redisRepo)(nil)

func (rr *redisRepo) RegisterProject(id, userID string) error {
	conn, err := rr.pool.Get()
	if err != nil {
		return err
	}
	defer rr.pool.Put(conn)

	// check to see if there is an existing project for this user.
	k := rr.key("projects")
	i, err := conn.Cmd("HEXISTS", k, userID).Int()
	if err != nil {
		return err
	}

	if i == 1 {
		return &ProjectExistsErr{}
	}

	_, err = conn.Cmd("HSET", k, userID, id).Int()
	if err != nil {
		return err
	}

	k = rr.key("users")
	_, err = conn.Cmd("HSET", k, id, userID).Int()
	if err != nil {
		return err
	}

	return nil
}

func (rr *redisRepo) ProjectID(userID string) (string, error) {
	conn, err := rr.pool.Get()
	if err != nil {
		return "", err
	}
	defer rr.pool.Put(conn)

	k := rr.key("projects")
	id, err := conn.Cmd("HGET", k, userID).Str()
	if err != nil {
		return "", err
	}

	return id, nil
}

func (rr *redisRepo) User(projectID string) (string, error) {
	conn, err := rr.pool.Get()
	if err != nil {
		return "", err
	}
	defer rr.pool.Put(conn)

	k := rr.key("users")
	id, err := conn.Cmd("HGET", k, projectID).Str()
	if err != nil {
		rr.log.WithError(err).
			WithFields(logrus.Fields{
			"key":       k,
			"projectID": projectID}).
			Error("fetch project id by user")
		return "", fmt.Errorf("fetch user by project id: %v", err)
	}

	return id, nil
}

func (rr *redisRepo) key(suffix string) string {
	return strings.Join([]string{rr.namespace, suffix}, keySeperator)
}
