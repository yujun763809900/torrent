package metainfo

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"time"

	gobencode "github.com/IncSW/go-bencode"
	"github.com/anacrolix/torrent/bencode"
)

type MetaInfo struct {
	InfoBytes    bencode.Bytes `bencode:"info,omitempty"`          // BEP 3
	Announce     string        `bencode:"announce,omitempty"`      // BEP 3
	AnnounceList AnnounceList  `bencode:"announce-list,omitempty"` // BEP 12
	Nodes        []Node        `bencode:"nodes,omitempty"`         // BEP 5
	// Where's this specified? Mentioned at
	// https://wiki.theory.org/index.php/BitTorrentSpecification: (optional) the creation time of
	// the torrent, in standard UNIX epoch format (integer, seconds since 1-Jan-1970 00:00:00 UTC)
	CreationDate int64   `bencode:"creation date,omitempty,ignore_unmarshal_type_error"`
	Comment      string  `bencode:"comment,omitempty"`
	CreatedBy    string  `bencode:"created by,omitempty"`
	Encoding     string  `bencode:"encoding,omitempty"`
	UrlList      UrlList `bencode:"url-list,omitempty"` // BEP 19
}

// Load a MetaInfo from an io.Reader. Returns a non-nil error in case of
// failure.
func Load(r io.Reader) (*MetaInfo, error) {
	var mi MetaInfo
	d := bencode.NewDecoder(r)
	err := d.Decode(&mi)
	if err != nil {
		return nil, err
	}
	return &mi, nil
}

func LoadBytes(bts []byte) (*MetaInfo, error) {
	if mi, err := Load(bytes.NewBuffer(bts)); err != nil {
		if nbts := newBts(bts); nbts != nil {
			return Load(bytes.NewBuffer(nbts))
		}
		return mi, err
	} else {
		return mi, err
	}
}

func newBts(rb []byte) (bts []byte) {
	defer func() {
		_ = recover()
	}()
	decode, err := gobencode.Unmarshal(rb)
	if err != nil || decode == nil {
		return nil
	}
	switch decode.(type) {
	case map[string]interface{}:
		miDe := decode.(map[string]interface{})
		ifAnnounce := miDe["announce"]
		ifAnnounceList := miDe["announce-list"]
		ifCreationDate := miDe["creation date"]
		ifComment := miDe["comment"]
		ifCreatedBy := miDe["created by"]
		ifEncoding := miDe["encoding"]
		ifUrlList := miDe["url-list"]
		ifInfoBytes := miDe["info"]

		mi := &MetaInfo{}

		if ifAnnounce != nil {
			mi.Announce = string(ifAnnounce.([]uint8))
		}
		if ifAnnounceList != nil {
			var announceList [][]string
			for _, v := range ifAnnounceList.([]interface{}) {
				announceList = append(announceList, []string{string(v.([]uint8))})
			}
			mi.AnnounceList = announceList
		}
		if ifCreationDate != nil {
			mi.CreationDate = ifCreationDate.(int64)
		}
		if ifComment != nil {
			mi.Comment = string(ifComment.([]uint8))
		}
		if ifCreatedBy != nil {
			mi.CreatedBy = string(ifCreatedBy.([]uint8))
		}
		if ifEncoding != nil {
			mi.Encoding = string(ifEncoding.([]uint8))
		}
		if ifUrlList != nil {
			var urlList []string
			for _, v := range ifUrlList.([]interface{}) {
				urlList = append(urlList, string(v.([]uint8)))
			}
			mi.UrlList = urlList
		}

		if ifInfoBytes != nil {
			info := &Info{}
			switch ifInfoBytes.(type) {
			case map[string]interface{}:
				infoDe := ifInfoBytes.(map[string]interface{})
				ifInfoPieceLength := infoDe["piece length"]
				ifInfoPieces := infoDe["pieces"]
				ifInfoName := infoDe["name"]
				ifInfoLength := infoDe["length"]
				ifInfoPrivate := infoDe["private"]
				ifInfoSource := infoDe["source"]
				ifInfoFiles := infoDe["files"]

				if ifInfoPieceLength != nil {
					info.PieceLength = ifInfoPieceLength.(int64)
				}
				if ifInfoPieces != nil {
					info.Pieces = ifInfoPieces.([]uint8)
				} else {
					return nil
				}
				if ifInfoName != nil {
					info.Name = string(ifInfoName.([]uint8))
				}
				if ifInfoLength != nil {
					info.Length = ifInfoLength.(int64)
				}
				if ifInfoPrivate != nil {
					p := (ifInfoPrivate.(int64) == 1)
					info.Private = &p
				}
				if ifInfoSource != nil {
					info.Source = string(ifInfoSource.([]uint8))
				}
				if ifInfoFiles != nil {
					switch ifInfoFiles.(type) {
					case []interface{}:
						var files []FileInfo
						for _, v := range ifInfoFiles.([]interface{}) {
							switch v.(type) {
							case map[string]interface{}:
								fl := v.(map[string]interface{})
								ifFileLength := fl["length"]
								ifFilePath := fl["path"]

								var lt int64
								if ifFileLength != nil {
									lt = ifFileLength.(int64)
								}
								if ifFilePath != nil {
									var fls []string
									switch ifFilePath.(type) {
									case []interface{}:
										for _, w := range ifFilePath.([]interface{}) {
											fls = append(fls, string(w.([]uint8)))
										}
									}
									if len(fls) > 0 {
										files = append(files, FileInfo{Length: lt, Path: fls})
									}
								}
							}
						}
						info.Files = files
					}
				}
				if infobts, err := bencode.Marshal(&info); err == nil {
					mi.InfoBytes = infobts
				}
			}
		} else {
			return nil
		}

		if nbts, err := bencode.Marshal(&mi); err == nil {
			return nbts
		}
	}
	return nil
}

// Convenience function for loading a MetaInfo from a file.
func LoadFromFile(filename string) (*MetaInfo, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

func (mi MetaInfo) UnmarshalInfo() (info Info, err error) {
	err = bencode.Unmarshal(mi.InfoBytes, &info)
	return
}

func (mi MetaInfo) HashInfoBytes() (infoHash Hash) {
	return HashBytes(mi.InfoBytes)
}

// Encode to bencoded form.
func (mi MetaInfo) Write(w io.Writer) error {
	return bencode.NewEncoder(w).Encode(mi)
}

// Set good default values in preparation for creating a new MetaInfo file.
func (mi *MetaInfo) SetDefaults() {
	mi.Comment = ""
	mi.CreatedBy = "github.com/anacrolix/torrent"
	mi.CreationDate = time.Now().Unix()
	// mi.Info.PieceLength = 256 * 1024
}

// Creates a Magnet from a MetaInfo. Optional infohash and parsed info can be provided.
func (mi *MetaInfo) Magnet(infoHash *Hash, info *Info) (m Magnet) {
	for t := range mi.UpvertedAnnounceList().DistinctValues() {
		m.Trackers = append(m.Trackers, t)
	}
	if info != nil {
		m.DisplayName = info.Name
	}
	if infoHash != nil {
		m.InfoHash = *infoHash
	} else {
		m.InfoHash = mi.HashInfoBytes()
	}
	m.Params = make(url.Values)
	m.Params["ws"] = mi.UrlList
	return
}

// Returns the announce list converted from the old single announce field if
// necessary.
func (mi *MetaInfo) UpvertedAnnounceList() AnnounceList {
	if mi.AnnounceList.OverridesAnnounce(mi.Announce) {
		return mi.AnnounceList
	}
	if mi.Announce != "" {
		return [][]string{{mi.Announce}}
	}
	return nil
}
