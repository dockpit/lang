package manifest

import (
	"encoding/json"
)

//
//
//
type Factory struct{}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) Draft(loc string) (*Manifest, error) {
	data, err := f.Load(loc)
	if err != nil {
		return nil, err
	}

	return NewManifest(data)
}

func (f *Factory) Load(loc string) (*ManifestData, error) {

	//create as link
	link, err := NewLink(loc)
	if err != nil {
		return nil, err
	}

	//creat reader
	r, err := link.Load()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	//decode into data struct
	c := &ManifestData{}
	dec := json.NewDecoder(r)
	err = dec.Decode(c)
	if err != nil {
		return nil, err
	}

	return c, nil
}
