package models

import (
	"encoding/binary"
	"strings"

	"github.com/qedus/nds"
	"golang.org/x/net/context"

	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
)

type Citizen struct {
	ID            int64  `datastore:"-"`
	Handle        string `datastore:",noindex"`
	Moniker       string `datastore:",noindex"`
	Organizations []string

	// HandleSearch is a variable used to search for this citizen by their
	// handle. It is automatically set by PutCitizen.
	HandleSearch   string
	originalHandle string `datastore:"-"`
}

func GetCitizen(c context.Context, id int64) (*Citizen, error) {
	var citizen Citizen

	err := nds.Get(c, generateCitizenKey(c, id), &citizen)
	if err == datastore.ErrNoSuchEntity {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	citizen.ID = id
	citizen.originalHandle = citizen.Handle

	return &citizen, nil
}

func PutCitizen(c context.Context, citizen *Citizen, historyItems []*CitizenHistory, saveCitizen bool) error {
	keys := make([]*datastore.Key, 0, len(historyItems)+1)
	toStore := make([]interface{}, 0, len(historyItems)+1)

	if saveCitizen {
		citizen.HandleSearch = strings.ToLower(citizen.Handle)

		keys = append(keys, generateCitizenKey(c, citizen.ID))
		toStore = append(toStore, citizen)
	}

	for _, historyItem := range historyItems {
		if historyItem.key == nil {
			historyItem.key = generateCitizenHistoryItemKey(c, citizen.ID)
		}

		keys = append(keys, historyItem.key)
		toStore = append(toStore, historyItem)
	}

	if len(keys) == 0 {
		return nil
	}

	newKeys, err := nds.PutMulti(c, keys, toStore)

	if newKeys != nil {
		if saveCitizen {
			newKeys = newKeys[1:]
		}

		// update the history items' keys
		for i, historyItem := range historyItems {
			historyItem.key = newKeys[i]
		}
	}

	if err == nil && citizen.Handle != citizen.originalHandle {
		// remove this citizen's handle from the handle-to-ID cache
		memcache.Delete(c, "handle-to-id:"+strings.ToLower(citizen.originalHandle))
		citizen.originalHandle = citizen.Handle
	}

	return err
}

func FindCitizenByHandle(c context.Context, handle string) (*Citizen, error) {
	handle = strings.ToLower(handle)

	memcacheItem, err := memcache.Get(c, "handle-to-id:"+handle)
	if err == nil {
		id, n := binary.Varint(memcacheItem.Value)
		if n > 0 {
			// ID was successfully read

			return GetCitizen(c, id)
		}
	}

	queryIterator := datastore.NewQuery("Citizen").Filter("HandleSearch=", handle).KeysOnly().Limit(1).Run(c)

	citizenKey, err := queryIterator.Next(nil)
	if err == datastore.Done {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	id := citizenKey.IntID()
	idEncoded := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(idEncoded, id)
	idEncoded = idEncoded[:n]
	memcache.Set(c, &memcache.Item{
		Key:   "handle-to-id:" + handle,
		Value: idEncoded,
	})

	return GetCitizen(c, id)
}

func generateCitizenKey(c context.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, "Citizen", "", id, nil)
}
