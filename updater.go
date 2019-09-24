package libupdate

import (
	"context"
	"flag"
	"fmt"
	"github.com/gen-iot/std"
	"github.com/pkg/errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"time"
)

type Session interface{}

// todo support freq policy
type Config struct {
	Frequency       time.Duration          // check update freq , required
	MainEntryPoint  func()                 // user main func entry point , required
	UpdateAvailable func() (download bool) // callback: update available, if not set, automatic download
	UpdateReady     func()                 // callback: means program should exit , optional
	WorkDir         string                 // where download file placed,if dir not exit, auto mk, default is .
	FileNamePolicy  NamePolicy             // how download file name

	// !!! advance !!!

	// program startup param name(bool value)
	// default is 'update'
	// updater will use check this param to determine whether itself subprocess
	StartupName1 string // todo support disable update
	// don't use `UpdateReady` callback, call os.Exit() to quit parent process instead
	GracefulExit bool
	// if true , will `ls -s old.exe new.exe`, instead of copy it
	UseLink bool
	// if set ,don't use os.Args[0], use `ExactlyExeAddr` name(must be a absolute path)
	ExactlyExeAddr string
}

type Updater interface {
	Provider() Repo
	Execute(ctx context.Context) error
}

type updaterImpl struct {
	conf     *Config
	pvd      Repo
	isParent bool
}

func NewUpdater(conf *Config, pvd Repo) Updater {
	upd := &updaterImpl{
		conf: conf,
		pvd:  pvd,
	}
	return upd
}

func (this *updaterImpl) doParent(ctx context.Context) (reader *os.File, err error) {
	this.isParent = true
	exeAddr := os.Args[0]
	// start update daemon
	paramName := kDefaultParamName
	if len(this.conf.StartupName1) != 0 {
		paramName = this.conf.StartupName1
	}
	cmd := exec.CommandContext(ctx, exeAddr, fmt.Sprintf("-%s=1", paramName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	parent, child, err := sockPair(true)
	if err != nil {
		return nil, errors.Wrap(err, "updater create pipe failed")
	}
	defer std.CloseIgnoreErr(child)
	cmd.ExtraFiles = []*os.File{child}
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "start updater daemon failed")
	}
	go func() {
		// wait and clean resource
		_ = cmd.Wait()
	}()
	go this.conf.MainEntryPoint()
	log.Println("start updater daemon success! pid=", cmd.Process.Pid)
	return parent, nil
}

func (this *updaterImpl) doChild(ctx context.Context) {
	comm := os.NewFile(uintptr(3), "|1_pipe")
	defer std.CloseIgnoreErr(comm)
	duration := this.conf.Frequency
	timer := time.NewTimer(duration)
	defer timer.Stop()
	upgradeTask := func() {
		session := this.pvd.NewSession()
		defer this.pvd.FreeSession(session)
		update, err := this.pvd.Check(ctx, session)
		defer timer.Reset(duration)
		if err != nil {
			log.Println("update daemon: check update failed,err->", err)
			return
		}
		if !update {
			log.Println("update daemon: no update available")
			return
		}
		shouldDownload := this.conf.UpdateAvailable == nil
		if !shouldDownload {
			shouldDownload = this.conf.UpdateAvailable()
		}
		if shouldDownload {
			newExeAddr := ""
			err = this.performDownload(ctx, session, &newExeAddr)
			if err != nil {
				log.Println("update daemon: download failed,err->", err)
				return
			}
			log.Println("update daemon: download success!")
			log.Println("update daemon: sending signal to parent!")
			_, _ = comm.Write([]byte{0})
			log.Println("update daemon: sending signal to parent!")
			_, _ = comm.Read([]byte{0})
			log.Println("update daemon: parent exit, now replace exe!")
			// replace it
			if recoverFn, err := this.performReplaceExe(newExeAddr); err != nil {
				log.Println("update daemon: replace failed err->", err)
				if recoverFn != nil {
					err := recoverFn()
					if err != nil {
						log.Println("update daemon: perform recovery failed,err->", err)
						return
					}
					log.Println("update daemon: perform recovery success")
				} else {
					log.Println("update daemon: unnecessary perform recover")
				}
			}
			err := this.rebootExe()
			if err != nil {
				log.Println("update daemon: reboot err->", err)
			}
			log.Println("update daemon:finish upgrade, exit")
			os.Exit(0)
		} else {
			log.Println("update daemon: download task canceled!")
		}
	}
	for {
		select {
		case <-timer.C:
			upgradeTask()
		case <-ctx.Done():
			return
		}
	}
}

var defaultPolicy NamePolicy = &TimeNamePolicy{Format: "2006-1-2-15-04-05"}

func (this *updaterImpl) rebootExe() error {
	exeAddr := os.Args[0]
	if len(this.conf.ExactlyExeAddr) != 0 {
		exeAddr = this.conf.ExactlyExeAddr
	}
	if err := os.Chmod(exeAddr, 755); err != nil {
		return err
	}
	cmd := exec.Command(exeAddr)
	return cmd.Start()
}
func (this *updaterImpl) performDownload(ctx context.Context, session Session, newFileAddr *string) error {
	namePolicy := this.conf.FileNamePolicy
	if namePolicy == nil {
		namePolicy = defaultPolicy
	}
	newFileName, err := namePolicy.GenerateName()
	if err != nil {
		return errors.Wrap(err, "generate name failed")
	}
	baseDir := "."
	if len(this.conf.WorkDir) != 0 {
		baseDir = this.conf.WorkDir
	}
	if info, err := os.Stat(baseDir); err != nil {
		if err := os.MkdirAll(baseDir, 0777|os.ModeDir); err != nil {
			return errors.Wrap(err, "create download dir failed")
		}
	} else if !info.IsDir() {
		return errors.Errorf("create download file failed,%s not a dir!", baseDir)
	}
	*newFileAddr = path.Join(baseDir, newFileName)
	file, err := os.OpenFile(*newFileAddr, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return errors.Wrap(err, "open/create download file failed")
	}
	defer std.CloseIgnoreErr(file)
	return this.pvd.Download(ctx, session, file)
}

func (this *updaterImpl) performReplaceExe(newExeAddr string) (recoverFn func() error, err error) {
	exeAddr := os.Args[0]
	if len(this.conf.ExactlyExeAddr) != 0 {
		exeAddr = this.conf.ExactlyExeAddr
	}
	useLink := this.conf.UseLink
	exeBak := fmt.Sprintf("%s.bak", exeAddr)
	if err := os.Rename(exeAddr, fmt.Sprintf(exeBak)); err != nil {
		return nil, errors.Wrap(err, "backup exe failed")
	}
	recoverFn = func() error {
		return os.Rename(exeBak, exeAddr)
	}
	if !useLink {
		output, err := os.OpenFile(exeAddr, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			return recoverFn, errors.Wrap(err, "create exe failed")
		}
		defer std.CloseIgnoreErr(output)
		input, err := os.Open(newExeAddr)
		if err != nil {
			return recoverFn, errors.Wrap(err, "read input new exe failed")
		}
		defer std.CloseIgnoreErr(input)
		if _, err := io.Copy(output, input); err != nil {
			return recoverFn, errors.Wrap(err, "copy new exe failed")
		}
	} else {
		if err := os.Symlink(newExeAddr, exeAddr); err != nil {
			return recoverFn, errors.Wrap(err, "symbol link failed")
		}
	}
	return recoverFn, nil

}

const kDefaultParamName = "update"

func (this *updaterImpl) Execute(ctx context.Context) error {
	paramName := kDefaultParamName
	if len(this.conf.StartupName1) != 0 {
		paramName = this.conf.StartupName1
	}
	isChild := false
	flagParser := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flagParser.BoolVar(&isChild, paramName, isChild, "is updater daemon")
	if err := flagParser.Parse(os.Args[1:]); err != nil {
		return errors.Wrap(err, "parse startup command line failed")
	}
	fmt.Println("is child ->", isChild)
	time.Sleep(time.Second * 1)
	if !isChild {
		reader, err := this.doParent(ctx)
		std.AssertError(err, "updater : parent failed!")
		defer std.CloseIgnoreErr(reader)
		buf := []byte{0}
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			_ = reader.SetReadDeadline(time.Now().Add(time.Second * 5))
			_, err = reader.Read(buf)
			if os.IsTimeout(err) {
				continue
			}
			log.Println("not timeout err???", err)
			if !this.conf.GracefulExit && this.conf.UpdateReady != nil {
				this.conf.UpdateReady()
			}
			os.Exit(0)
		}
	} else {
		this.doChild(ctx)
	}
	return nil
}

func (this *updaterImpl) Provider() Repo {
	return this.pvd
}