package topten

import "time"

type ListID string

type TopTenList struct {
	Date  string   `json:"date"`
	Title string   `json:"title"`
	Items []string `json:"items"`
	Year  int      `json:"year"`
	Show  string   `json:"show"`
	URL   string   `json:"url"`
}

type TopTenCollection struct {
	Lists []TopTenList `json:"lists"`
}

type ListSelector interface {
	GetRandomList() (TopTenList, error)
}

type Config struct {
	RandomSeed int64
}

func NewConfig() Config {
	return Config{
		RandomSeed: time.Now().UnixNano(),
	}
}
