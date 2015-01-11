package manifest

//
//
// represents a service manifest
type Manifest struct {
	name      string
	resources []R
}

func NewManifest(data *ManifestData) (*Manifest, error) {

	res := []R{}
	for _, r := range data.Resources {

		cases := []*Pair{}
		for _, c := range r.Cases {

			//create pair from data
			p, err := NewPairFromData(c, data)
			if err != nil {
				return nil, err
			}

			cases = append(cases, p)
		}

		res = append(res, NewResource(r.Pattern, cases...))
	}

	return &Manifest{name: data.Name, resources: res}, nil
}

func (c *Manifest) Name() string {
	return c.name
}

func (c *Manifest) Resources() ([]R, error) {
	return c.resources, nil
}

// walk resources, actions and pairs to map all necessary states
// to test against the manifest
func (c *Manifest) States() (map[string][]string, error) {
	states := map[string][]string{}

	res, err := c.Resources()
	if err != nil {
		return states, err

	}

	for _, r := range res {
		as, err := r.Actions()
		if err != nil {
			return states, err
		}

		//loop over all pairs to map states
		for _, a := range as {
			for _, p := range a.Pairs() {
				for pname, g := range p.Given {

					//add state
					if _, ok := states[pname]; !ok {
						states[pname] = []string{}
					}

					//@todo prevend duplicate snames?
					states[pname] = append(states[pname], g.Name)

				}
			}
		}
	}

	return states, nil
}

// walk resources, actions and pairs to map all necessary dependencies
// to be mocked for isolation
func (c *Manifest) Dependencies() (map[string][]string, error) {
	deps := map[string][]string{}

	res, err := c.Resources()
	if err != nil {
		return deps, err
	}

	for _, r := range res {
		as, err := r.Actions()
		if err != nil {
			return deps, err
		}

		//loop over all pairs to map deps
		for _, a := range as {
			for _, p := range a.Pairs() {
				for _, w := range p.While {
					if _, ok := deps[w.ID]; !ok {
						deps[w.ID] = []string{}

						//@todo also add casenames?
					}
				}
			}
		}
	}

	return deps, nil
}
