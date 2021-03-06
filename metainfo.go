package main

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/nictuku/dht"
	"github.com/zeebo/bencode"
)

type FileDict struct {
	Length int64    `bencode:"length"`
	Path   []string `bencode:"path"`
	Md5sum string   `bencode:"md5sum,omitempty"`
}

type InfoDict struct {
	PieceLength int64  `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`
	Private     int64  `bencode:"private"`
	Name        string `bencode:"name"`
	// Single File Mode
	Length int64  `bencode:"length,omitempty"`
	Md5sum string `bencode:"md5sum,omitempty"`
	// Multiple File mode
	Files []*FileDict `bencode:"files,omitempty"`
}

type MetaInfo struct {
	Info         *InfoDict  `bencode:"info"`
	Announce     string     `bencode:"announce,omitempty"`
	AnnounceList [][]string `bencode:"announce-list,omitempty"`
	CreationDate int64      `bencode:"creation date,omitempty"`
	Comment      string     `bencode:"comment,omitempty"`
	CreatedBy    string     `bencode:"created by,omitempty"`
	Encoding     string     `bencode:"encoding,omitempty"`

	// These are not used for bencoding, only for helping
	InfoHash string `bencode:"-"`
	rawInfo  []byte `bencode:"-"`
}

func NewMetaInfo(torrent string) (m *MetaInfo, err error) {
	if strings.HasPrefix(torrent, "http:") {
		return NewMetaInfoFromHTTP(torrent)
	} else if strings.HasPrefix(torrent, "magnet:") {
		return NewMetaInfoFromMagnet(torrent)
	} else {
		m, err := NewMetaInfoFromContent([]byte(torrent))
		if err == nil {
			return m, nil
		}
		return NewMetaInfoFromFile(torrent)
	}
}

func NewMetaInfoFromHTTP(torrent string) (m *MetaInfo, err error) {
	r, err := proxyHttpGet(torrent)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}

	return NewMetaInfoFromContent(content)
}

func NewMetaInfoFromFile(torrent string) (m *MetaInfo, err error) {
	content, err := ioutil.ReadFile(torrent)
	if err != nil {
		return
	}

	return NewMetaInfoFromContent(content)
}

func NewMetaInfoFromContent(content []byte) (m *MetaInfo, err error) {

	var m1 MetaInfo
	err1 := bencode.DecodeString(string(content), &m1)
	if err1 != nil {
		err = errors.New("Couldn't parse torrent file: " + err1.Error())
		return
	}

	hash := sha1.New()
	err1 = bencode.NewEncoder(hash).Encode(m1.Info)
	if err1 != nil {
		return
	}

	m1.InfoHash = string(hash.Sum(nil))

	return &m1, nil
}

func NewMetaInfoFromMagnet(torrent string) (m *MetaInfo, err error) {
	magnet, err := parseMagnet(torrent)
	if err != nil {
		log.Println("Couldn't parse magnet: ", err)
		return
	}

	ih, err := dht.DecodeInfoHash(magnet.InfoHashes[0])
	if err != nil {
		return
	}

	m = &MetaInfo{InfoHash: string(ih)}
	return

}

func (m *MetaInfo) saveToDisk(dir string) (err error) {
	ihhex := fmt.Sprintf("%x", m.InfoHash)
	f, err := os.Create(filepath.Join(dir, ihhex))
	if err != nil {
		log.Println("Error when opening file for creation: ", err)
		return
	}
	defer f.Close()

	return bencode.NewEncoder(f).Encode(m)
}

// Returns the size of the representation of this metainfo as a bencoded
// dictionary
func (m *MetaInfo) Size() (sz int) {
	var buf bytes.Buffer
	err := bencode.NewEncoder(&buf).Encode(m)
	if err != nil {
		log.Fatal("Couldn't bencode this metainfo: ", err)
	}

	return buf.Len()
}

// Returns the representation of this metainfo's info dict as a
// bencoded dictionary
func (m *MetaInfo) RawInfo() (b []byte) {
	if m.rawInfo != nil {
		return m.rawInfo
	}
	if m.Info == nil {
		return []byte{}
	}

	var buf bytes.Buffer
	err := bencode.NewEncoder(&buf).Encode(m.Info)
	if err != nil {
		log.Fatal("Couldn't bencode this metainfo's dict: ", err)
	}

	m.rawInfo = buf.Bytes()

	return m.rawInfo
}

func getMetaInfo(torrent string) (metaInfo *MetaInfo, err error) {
	return NewMetaInfo(torrent)
}

type SessionInfo struct {
	PeerId     string
	Port       int
	Uploaded   int64
	Downloaded int64
	Left       int64

	//UseDHT      bool
	FromMagnet  bool
	HaveTorrent bool

	OurExtensions map[int]string
	ME            *MetaDataExchange
}

type MetaDataExchange struct {
	Transferring bool
	Pieces       [][]byte
}
