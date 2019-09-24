package libupdate

import (
	"context"
	"io"
)

type Repo interface {
	Check(ctx context.Context, session Session) (update bool, err error)
	Download(ctx context.Context, session Session, writer io.Writer) error
	NewSession() Session
	FreeSession(session Session)
}
