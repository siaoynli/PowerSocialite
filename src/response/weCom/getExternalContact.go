package weCom

import "github.com/ArtisanCloud/PowerSocialite/src/models"

type ResponseGetExternalContact struct {
	*ResponseWeCom
	*models.ExternalContact `json:"external_contact"`
	FollowInfo      []*models.FollowUser                  `json:"follow_user"`
}

