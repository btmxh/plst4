package media

import (
	"database/sql"
	"fmt"
	"strings"
)

func GetThumbnail(url sql.NullString) string {
	if url.Valid {
		if id, found := strings.CutPrefix(url.String, "https://youtu.be/"); found {
			return fmt.Sprintf("https://i3.ytimg.com/vi/%s/maxresdefault.jpg", id)
		}
	}

	return "/assets/local.svg"
}
