package rdapi

type Torrent struct {
	ID       string `json:"id"`
	Hash     string `json:"hash"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
}

type addMagnetResponse struct {
	ID  string `json:"id"`
	URI string `json:"uri"`
}
