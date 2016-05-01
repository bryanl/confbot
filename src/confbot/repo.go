package confbot

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/mediocregopher/radix.v2/pool"
	"github.com/mediocregopher/radix.v2/redis"
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
	Ping() error
	RegisterProject(id, userID string) error
	ResetProject(userID string) error
	ProjectID(userID string) (string, error)
	User(projectID string) (string, error)
	SaveKey(projectID string, privateKey []byte) error
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

	if err := repo.Ping(); err != nil {
		return nil, err
	}

	return repo, nil
}

type redisRepo struct {
	pool      *pool.Pool
	namespace string
	log       *logrus.Entry
}

var _ Repo = (*redisRepo)(nil)

type redisCmd func(*redis.Client) error

func (rr *redisRepo) Ping() error {
	conn, err := rr.pool.Get()
	if err != nil {
		return err
	}

	defer rr.pool.Put(conn)

	res, err := conn.Cmd("PING").Str()
	if err != nil {
		return err
	}

	if res != "PONG" {
		return fmt.Errorf("unexpected PING reply from redis: %s", res)
	}

	return nil
}

func (rr *redisRepo) ResetProject(userID string) error {
	conn, err := rr.pool.Get()
	if err != nil {
		return err
	}
	defer rr.pool.Put(conn)

	log := rr.log.WithField("user-id", userID)

	projectsKey := rr.key("projects")
	log.Info("deleting user entry")
	if _, err = conn.Cmd("HDEL", projectsKey, userID).Int(); err != nil {
		log.WithError(err).Error("unable to delete project entry")
		return err
	}

	usersKey := rr.key("users")
	m, err := conn.Cmd("HGETALL", usersKey).Map()
	if err != nil {
		return err
	}

	for k, v := range m {
		log := log.WithField("project-id", v)
		if v == userID {
			log.Info("deleting project entry")
			if i, err := conn.Cmd("HDEL", usersKey, k).Int(); err != nil {
				log.WithField("items-deleted", i).WithError(err).Error("unable to delete project")
				return err
			}
		}
	}

	return nil
}

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

func (rr *redisRepo) SaveKey(projectID string, privateKey []byte) error {
	conn, err := rr.pool.Get()
	if err != nil {
		return err
	}
	defer rr.pool.Put(conn)

	k := rr.key("keys")
	_, err = conn.Cmd("HSET", k, projectID, privateKey).Int()

	return err
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
