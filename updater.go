package libupdate

import (
	"context"
	"github.com/gen-iot/std"
	"github.com/pkg/errors"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"
	"time"
)

type Session interface{}

type Config struct {
	// check update freq , required
	Frequency time.Duration
	// callback: update available, if not set, automatic download
	UpdateAvailable func() (download bool)
	// callback: download latest success
	UpdateReady func(updater Updater, latestExeAddr string)
	// where download file placed,if dir not exit, auto mk, default is ./download
	DownloadDir string
	// how download file name
	FileNamePolicy NamePolicy
}

type Updater interface {
	Provider() Repo
	Execute(ctx context.Context)
	Active(enable bool)
}

type updaterImpl struct {
	conf     *Config
	repo     Repo
	disabled int32
}

func NewUpdater(conf *Config, repo Repo) Updater {
	upd := &updaterImpl{
		conf: conf,
		repo: repo,
	}
	return upd
}

func (this *updaterImpl) Active(enable bool) {
	if enable {
		atomic.StoreInt32(&this.disabled, 0)
	} else {
		atomic.StoreInt32(&this.disabled, 1)
	}
}

var defaultPolicy NamePolicy = &TimeNamePolicy{Format: "2006-1-2-15-04-05"}
var defaultDownloadPath = "download" + string(filepath.Separator)

func (this *updaterImpl) performDownload(ctx context.Context, session Session, newFileAddr *string) (err error) {
	namePolicy := this.conf.FileNamePolicy
	if namePolicy == nil {
		namePolicy = defaultPolicy
	}
	newFileName, err := namePolicy.GenerateName()
	if err != nil {
		return errors.Wrap(err, "generate name failed")
	}
	baseDir := defaultDownloadPath
	if len(this.conf.DownloadDir) != 0 {
		baseDir = this.conf.DownloadDir
	}
	if info, err := os.Stat(baseDir); err != nil {
		if err := os.MkdirAll(baseDir, 0755|os.ModeDir); err != nil {
			return errors.Wrap(err, "create download dir failed")
		}
	} else if !info.IsDir() {
		return errors.Errorf("create download file failed,%s not a dir!", baseDir)
	}
	*newFileAddr = path.Join(baseDir, newFileName)
	file, err := os.OpenFile(*newFileAddr, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0755)
	if err != nil {
		return errors.Wrap(err, "open/create download file failed")
	}
	defer func() {
		std.CloseIgnoreErr(file)
		if err != nil {
			_ = os.Remove(*newFileAddr)
		}
	}()
	err = this.repo.Download(ctx, session, file)
	if err != nil {
		return err
	}
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	err = this.repo.ValidateDownload(ctx, session, file)
	return err
}

func (this *updaterImpl) updateTask(ctx context.Context) {
	session := this.repo.NewSession()
	defer this.repo.FreeSession(session)
	update, err := this.repo.CheckUpdate(ctx, session)

	if err != nil {
		log.Println("updater: check update failed,err->", err)
		return
	}
	if !update {
		log.Println("updater: no update available :D")
		return
	}
	shouldDownload := this.conf.UpdateAvailable == nil
	if !shouldDownload {
		shouldDownload = this.conf.UpdateAvailable()
	}
	newExeAddr := ""
	if shouldDownload {
		err = this.performDownload(ctx, session, &newExeAddr)
		if err != nil {
			log.Println("updater: download failed,err->", err)
			return
		}
	} else {
		log.Println("updater: download task canceled!")
	}
	if this.conf.UpdateReady != nil {
		this.conf.UpdateReady(this, newExeAddr)
	}
}

func (this *updaterImpl) Execute(ctx context.Context) {
	duration := this.conf.Frequency
	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			if atomic.LoadInt32(&this.disabled) != 1 {
				this.updateTask(ctx)
			} else {
				log.Println("update: update have been disabled...")
			}
			timer.Reset(duration)
		case <-ctx.Done():
			return
		}
	}
}

func (this *updaterImpl) Provider() Repo {
	return this.repo
}
