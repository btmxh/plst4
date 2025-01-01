package stores

import "github.com/gin-gonic/gin"

const PlaylistIdKey = "playlist-id"

func SetPlaylistId(c *gin.Context, id int) {
	c.Set(PlaylistIdKey, id)
}

func GetPlaylistId(c *gin.Context) int {
	if value, ok := c.Get(PlaylistIdKey); ok && value != nil {
		id, ok := value.(int)
		if ok {
			return id
		}
	}

	panic("Playlist ID not set, please check the usage of SetPlaylistId")
}
