package libupdate

import (
	"context"
	"io"
)

type Repo interface {
	CheckUpdate(ctx context.Context, session Session) (update bool, err error)
	Download(ctx context.Context, session Session, writer io.Writer) error
	ValidateDownload(ctx context.Context, session Session, reader io.Reader) error
	NewSession() Session
	FreeSession(session Session)
}
