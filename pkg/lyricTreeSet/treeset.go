package lyricTreeSet

import (
	"fmt"
	"github.com/emirpasic/gods/lists/singlylinkedlist"
	"github.com/emirpasic/gods/maps/hashmap"
)

type LyricsSet struct {
	sizeLimit		int
	hmap			*hashmap.Map
	linkedList		*singlylinkedlist.List
}


type metaLyric struct {
	artist, title		string
}

// Creates the underlying structures of
// LyricsSet - hashmap and linked list.
func New(sizeLimit int) *LyricsSet {
	hmap := hashmap.New()
	ll := singlylinkedlist.New()
	return &LyricsSet{sizeLimit, hmap, ll}
}

// Put adds key and value to LyricSet's hashmap and adds key
// to the end of the linked list. It checks if the size of
// the set exceeded the limit, in which case it gets the
// first element of linked list and removes it from both
// hashmap and linked list.
func (lset *LyricsSet) Put(keyArtist, keyTitle string, value string) {
	key := metaLyric{keyArtist, keyTitle}
	lset.hmap.Put(key, value)
	lset.linkedList.Add(key)

	if lset.linkedList.Size() > lset.sizeLimit {
		el, ok := lset.linkedList.Get(0)
		if !ok {
			panic("List out of bounds")
		}
		oldest := el.(metaLyric)
		lset.hmap.Remove(oldest)
		lset.linkedList.Remove(0)
		fmt.Printf("Removed oldest lyric: %s - %s\n", oldest.artist, oldest.title)
	}
}

// Get gets the value (the lyrics string) from the
// hashmap, and relocates the metaLyric from the linked list
// to the end of the list.
func (lset *LyricsSet) Get(artist, title string) (string, bool) {
	key	:= metaLyric{artist, title}
	el, ok := lset.hmap.Get(key)
	if !ok {
		return "", false
	}

	// find in linked list, remove, and add to the end.
	it := lset.linkedList.Iterator()
	for it.Next() {
		i, v := it.Index(), it.Value()
		if v == key {
			lset.linkedList.Remove(i)
			lset.linkedList.Add(key)
		}
	}

	return el.(string), true
}