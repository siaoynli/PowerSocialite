package providers

import (
	"errors"
	"fmt"
	"github.com/ArtisanCloud/PowerLibs/object"
	"github.com/ArtisanCloud/PowerSocialite/src"
	"github.com/ArtisanCloud/PowerSocialite/src/exceptions"
	"github.com/ArtisanCloud/PowerSocialite/src/response/weCom"
)

const NAME = "wecom"

type WeCom struct {
	*Base

	detailed       bool
	agentId        int
	apiAccessToken string
}

func NewWeCom(config *object.HashMap) *WeCom {
	wecom := &WeCom{
		Base: NewBase(config),
	}

	wecom.OverrideGetAuthURL()
	wecom.OverrideGetTokenURL()
	wecom.OverrideGetUserByToken()
	wecom.OverrideMapUserToObject()

	return wecom
}

func (provider *WeCom) SetAgentID(agentId int) *WeCom {
	provider.agentId = agentId

	return provider
}

func (provider *WeCom) UserFromCode(code string) (*src.User, error) {
	token, err := provider.GetAPIAccessToken()
	if err != nil {
		return nil, err
	}

	userInfo, err := provider.GetUserID(token, code)
	if err != nil {
		return nil, err
	}
	var (
		user       *src.User
		userDetail *weCom.ResponseGetUserByID
	)

	if provider.detailed {
		userDetail, err = provider.GetUserByID(userInfo.UserID)
		if err != nil {
			return nil, err
		}
		user = provider.MapUserToObject(userDetail)
	} else {
		user = provider.MapUserToObject(userInfo)
	}

	return user.SetProvider(provider).SetRaw(*user.GetAttributes()), nil
}

func (provider *WeCom) ContactFromCode(code string) (*src.User, error) {
	token, err := provider.GetAPIAccessToken()
	if err != nil {
		return nil, err
	}

	userInfo, err := provider.GetUserID(token, code)
	if err != nil {
		return nil, err
	}
	var (
		user       *src.User
		userDetail *weCom.ResponseGetUserByID
	)

	if provider.detailed {
		userDetail, err = provider.GetUserByID(userInfo.UserID)
		if err != nil {
			return nil, err
		}
		user = provider.Detailed().MapUserToContact(userDetail)
	} else {
		user = provider.MapUserToContact(userInfo)
	}

	return user.SetProvider(provider).SetRaw(*user.GetAttributes()), nil
}

func (provider *WeCom) Detailed() *WeCom {
	provider.detailed = true

	return provider
}

func (provider *WeCom) WithApiAccessToken(apiAccessToken string) *WeCom {

	provider.apiAccessToken = apiAccessToken

	return provider
}

func (provider *WeCom) getOAuthURL() string {
	queries := &object.StringMap{
		"appid":         provider.GetClientID(),
		"redirect_uri":  provider.redirectURL,
		"response_type": "code",
		"scope":         provider.formatScopes(provider.scopes, provider.scopeSeparator),
		"state":         provider.state,
	}
	strQueries := object.ConvertStringMapToString(queries, "&")
	strQueries = "https://open.weixin.qq.com/connect/oauth2/authorize?" + strQueries + "#wechat_redirect"
	return strQueries
}

func (provider *WeCom) GetQrConnectURL() (string, error) {
	strAgentID := provider.agentId
	if strAgentID == 0 {
		strAgentID = provider.config.Get("agent_id", 0).(int)
		if strAgentID == 0 {
			return "", errors.New(fmt.Sprintf("You must config the `agentid` in configuration or using `setAgentid(%d)`.", strAgentID))
		}
	}

	queries := &object.StringMap{
		"appid":        provider.GetClientID(),
		"agentid":      fmt.Sprintf("%d", strAgentID),
		"redirect_uri": provider.redirectURL,
		"state":        provider.state,
	}
	strQueries := object.ConvertStringMapToString(queries, "&")
	strQueries = "https://open.work.weixin.qq.com/wwopen/sso/qrConnect?" + strQueries + "#wechat_redirect"
	return strQueries, nil
}

func (provider *WeCom) GetAPIAccessToken() (result string, err error) {
	if provider.apiAccessToken == "" {
		provider.apiAccessToken, err = provider.createApiAccessToken()
		if err != nil {
			return "", err
		}
	}
	return provider.apiAccessToken, nil
}

func (provider *WeCom) GetUserID(token string, code string) (*weCom.ResponseGetUserInfo, error) {

	outResponse := &weCom.ResponseGetUserInfo{}
	provider.GetHttpClient().PerformRequest(
		"https://qyapi.weixin.qq.com/cgi-bin/user/getuserinfo",
		"GET",
		&object.HashMap{
			"query": &object.StringMap{
				"access_token": token,
				"code":         code,
			},
		},
		false, nil,
		outResponse,
	)
	if outResponse.ErrCode > 0 || (outResponse.UserID == "" && outResponse.DeviceID == "" && outResponse.OpenID == "") {
		defer exceptions.NewAuthorizeFailedException().HandleException(nil, "base.get.userID", outResponse)
		if outResponse.ErrMSG == "" {
			outResponse.ErrMSG = "unknow"
		}
		return nil, errors.New(fmt.Sprintf("Failed to get user openid:%s", outResponse.ErrMSG))
	} else if outResponse.UserID == "" {
		provider.detailed = false
	}
	return outResponse, nil
}

func (provider *WeCom) GetUserByID(userID string) (*weCom.ResponseGetUserByID, error) {

	outResponse := &weCom.ResponseGetUserByID{}
	strAPIAccessToken, err := provider.GetAPIAccessToken()
	if err != nil {
		return nil, err
	}
	provider.GetHttpClient().PerformRequest(
		"https://qyapi.weixin.qq.com/cgi-bin/user/get",
		"POST",
		&object.HashMap{
			"query": &object.StringMap{
				"access_token": strAPIAccessToken,
				"userid":       userID,
			},
		},
		false, nil,
		outResponse,
	)
	if outResponse.ErrCode > 0 || outResponse.UserID == "" {
		defer (&exceptions.AuthorizeFailedException{}).HandleException(nil, "base.refresh.token", outResponse)
		if outResponse.ErrMSG == "" {
			outResponse.ErrMSG = "unknow"
		}
		return nil, errors.New(fmt.Sprintf("Failed to get user:%s", outResponse.ErrMSG))
	}

	return outResponse, nil
}

func (provider *WeCom) createApiAccessToken() (string, error) {
	outResponse := &weCom.ResponseTokenFromCode{}

	var (
		corpID     string = provider.config.Get("corpid", "").(string)
		corpSecret string = provider.config.Get("corpsecret", "").(string)
	)
	pCorpID := provider.config.Get("corp_id", nil)
	pCorpSecret := provider.config.Get("corp_secret", nil)

	if pCorpID != nil && pCorpID.(string) != "" {
		corpID = pCorpID.(string)
	}
	if pCorpSecret != nil && pCorpSecret.(string) != "" {
		corpSecret = pCorpSecret.(string)
	}

	provider.GetHttpClient().PerformRequest(
		"https://qyapi.weixin.qq.com/cgi-bin/gettoken",
		"GET",
		&object.HashMap{
			"query": &object.StringMap{
				"corpid":     corpID,
				"corpsecret": corpSecret,
			},
		},
		false, nil,
		outResponse,
	)
	if outResponse.ErrCode > 0 {
		defer (&exceptions.AuthorizeFailedException{}).HandleException(nil, "base.refresh.token", outResponse)
		if outResponse.ErrMSG == "" {
			outResponse.ErrMSG = "unknow"
		}
		return "", errors.New(fmt.Sprintf("Failed to get api access_token:%s", outResponse.ErrMSG))
	}
	return outResponse.AccessToken, nil

}

func (provider *WeCom) IdentifyUserAsEmployee(user *src.User) (userID string) {
	userID = user.GetAttribute("userID", "").(string)

	return userID

}

func (provider *WeCom) IdentifyUserAsContact(user *src.User) (openID string) {
	openID = user.GetAttribute("openID", "").(string)

	return openID
}

// Override GetCredentials
func (provider *WeCom) OverrideGetAuthURL() {
	provider.GetAuthURL = func() (string, error) {
		// 网页授权登录
		if len(provider.scopes) > 0 {
			return provider.getOAuthURL(), nil
		}

		// 第三方网页应用登录（扫码登录）
		return provider.GetQrConnectURL()
	}
}
func (provider *WeCom) OverrideGetTokenURL() {
	provider.GetTokenURL = func() string {
		return ""
	}
}
func (provider *WeCom) OverrideGetUserByToken() {
	provider.GetUserByToken = func(token string) (*object.HashMap, error) {

		return nil, errors.New("WeCom doesn't support access_token mode")
	}
}

func (provider *WeCom) OverrideMapUserToObject() {

	provider.MapUserToObject = func(userData interface{}) *src.User {

		if provider.detailed {
			// weCom.ResponseGetUserByID is detail response
			MapUser, _ := object.StructToHashMap(userData)
			return src.NewUser(MapUser, provider)
		}

		// weCom.ResponseGetUserInfo is response from code to user
		userInfo := userData.(*weCom.ResponseGetUserInfo)
		return src.NewUser(&object.HashMap{
			"userID":   userInfo.UserID,
			"deviceID": userInfo.DeviceID,
			"openID":   userInfo.OpenID,
		}, provider)
	}
}

func (provider *WeCom) MapUserToEmployee(userData interface{}) *src.User {
	return provider.MapUserToObject(userData)
}

func (provider *WeCom) MapUserToContact(userData interface{}) *src.User {

	if provider.detailed {
		MapUser, _ := object.StructToHashMap(userData)
		return src.NewUser(MapUser, provider)
	}

	userInfo := userData.(*weCom.ResponseGetUserInfo)
	return src.NewUser(&object.HashMap{
		"userID":         userInfo.UserID,
		"deviceID":       userInfo.DeviceID,
		"openID":         userInfo.OpenID,
		"externalUserID": userInfo.ExternalUserID,
	}, provider)

}