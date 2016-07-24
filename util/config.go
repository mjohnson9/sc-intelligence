package util

type ConfigItem struct {
	_kind string `goon:"kind,Config"`
	ID    string `datastore:"-" goon:"id"`
	Value string `datastore:",noindex"`
}

/*func GetAPIKey(c *Context) (string, error) {
	config := &ConfigItem{
		ID: "XenForoAPIKey",
	}
	if err := c.Goon.Get(config); err == datastore.ErrNoSuchEntity {
		if _, err := c.Goon.Put(config); err != nil {
			return "", err
		}
		return "", nil
	} else if err != nil {
		return "", err
	}

	return config.Value, nil
}

func GetSpreadsheetID(c *Context) (string, error) {
	config := &ConfigItem{
		ID: "SpreadsheetID",
	}
	if err := c.Goon.Get(config); err == datastore.ErrNoSuchEntity {
		if _, err := c.Goon.Put(config); err != nil {
			return "", err
		}
		return "", nil
	} else if err != nil {
		return "", err
	}

	return config.Value, nil
}*/
