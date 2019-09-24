package libupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gen-iot/std"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
)

type SimpleRepo struct {
	BaseUrl    string
	CurrentVer string
	RepoName   string
}

type simpleSession struct {
	latest *LatestResponse
}

type LatestResponse struct {
	Version string `json:"version" validate:"required"`
	URL     string `json:"url" validate:"required"`
	MD5     string `json:"md5" validate:"required"`
}

func (this *SimpleRepo) Check(ctx context.Context, session Session) (update bool, err error) {
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

func (this *SimpleRepo) NewSession() Session {
	return new(simpleSession)
}

func (this *SimpleRepo) FreeSession(session Session) {
}
