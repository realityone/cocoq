package dbcmd

import (
	"context"

	"cocoq/server/database"
	"cocoq/server/database/dbrt"
	"cocoq/utils"

	"github.com/pkg/errors"
)

const initialUserState int64 = 0

func initRootUser(ctx context.Context, dbPath string) (*dbrt.User, error) {
	client, err := database.OpenClient(dbPath)
	if err != nil {
		return nil, errors.Wrap(err, "open database")
	}
	defer client.Close()

	_, err = client.User.Get(ctx, 1)
	switch {
	case err == nil:
		return nil, errors.New("root user already initialized")
	case !dbrt.IsNotFound(err):
		return nil, errors.Wrap(err, "check root user")
	}

	oauthStateToken, err := utils.GenerateOAuthState()
	if err != nil {
		return nil, errors.Wrap(err, "generate oauth state token")
	}

	user, err := client.User.
		Create().
		SetOauthStateToken(oauthStateToken).
		SetState(initialUserState).
		Save(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "create root user")
	}

	return user, nil
}
