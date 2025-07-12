package stores

import "github.com/gin-gonic/gin"

const MediaIdKey = "media-id"

func SetMediaId(c *gin.Context, id int) {
	c.Set(MediaIdKey, id)
}

func GetMediaId(c *gin.Context) int {
	if value, ok := c.Get(MediaIdKey); ok && value != nil {
		id, ok := value.(int)
		if ok {
			return id
		}
	}

	panic("Media ID not set, please check the usage of SetMediaId")
}
