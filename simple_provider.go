package libupdate

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/gen-iot/std"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type SimpleRepo struct {
	BaseUrl    string `json:"baseUrl"`
	CurrentVer string `json:"currentVersion"`
	RepoName   string `json:"repoName"`
}

type simpleSession struct {
	latest *LatestResponse
}

type LatestResponse struct {
	Version string `json:"version" validate:"required"`
	URL     string `json:"url" validate:"required"`
	MD5     string `json:"md5" validate:"required"`
}

func (this *SimpleRepo) CheckUpdate(ctx context.Context, session Session) (update bool, err error) {
	url := fmt.Sprintf("%s/%s/latest", this.BaseUrl, this.RepoName)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return false, err
	}
	defer std.CloseIgnoreErr(response.Body)
	if response.StatusCode != http.StatusOK {
		return false, errors.Errorf("request failed , status code = %s", response.Status)
	}
	rspBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return false, err
	}
	obj := new(LatestResponse)
	if err = json.Unmarshal(rspBody, obj); err != nil {
		return false, err
	}
	err = std.ValidateStruct(obj)
	if err != nil {
		return false, err
	}
	session.(*simpleSession).latest = obj
	return true, nil
}

func (this *SimpleRepo) Download(ctx context.Context, session Session, writer io.Writer) error {
	ss := session.(*simpleSession)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, ss.latest.URL, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer std.CloseIgnoreErr(response.Body)
	if response.StatusCode != http.StatusOK {
		return errors.New(response.Status)
	}
	_, err = io.Copy(writer, response.Body)
	if err != nil {
		return err
	}
	return nil
}

func (this *SimpleRepo) ValidateDownload(ctx context.Context, session Session, reader io.Reader) error {
	ss := session.(*simpleSession)
	hash := md5.New()
	_, err := io.Copy(hash, reader)
	if err != nil {
		return err
	}
	md5Result := strings.ToLower(std.ArrToHexStrWithSp(hash.Sum(nil), ""))
	realMd5 := strings.ToLower(ss.latest.MD5)
	if strings.Compare(realMd5, md5Result) != 0 {
		return errors.Errorf("md5 should be %s , not %s", realMd5, md5Result)
	}
	return nil
}

func (this *SimpleRepo) NewSession() Session {
	return new(simpleSession)
}

func (this *SimpleRepo) FreeSession(session Session) {
}
