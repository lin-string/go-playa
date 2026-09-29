package parser

const invalidStreamFilter = "\x00invalid"

func StreamFilters(d Dict) ([]string, []Dict) {
	return StreamFiltersWithResolver(d, func(o Object) Object { return o })
}

func StreamFiltersWithResolver(d Dict, resolve func(Object) Object) ([]string, []Dict) {
	resolveChain := func(value Object) Object {
		seen := map[Ref]bool{}
		for {
			ref, ok := value.(Ref)
			if !ok {
				return value
			}
			if seen[ref] {
				return value
			}
			seen[ref] = true
			resolved := resolve(value)
			if resolved == nil {
				return resolved
			}
			value = resolved
		}
	}
	var filters []string
	var parameters []Dict
	filter, present := d[Name("Filter")]
	if !present {
		filter, present = d[Name("F")]
	}
	if present {
		filter = resolveChain(filter)
	}
	switch value := filter.(type) {
	case Name:
		filters = []string{string(value)}
	case Array:
		for _, item := range value {
			item = resolveChain(item)
			name, ok := item.(Name)
			if !ok {
				filters = []string{invalidStreamFilter}
				break
			}
			filters = append(filters, string(name))
		}
	default:
		if present {
			filters = []string{invalidStreamFilter}
		}
	}
	params, paramsPresent := d[Name("DecodeParms")]
	if !paramsPresent {
		params, paramsPresent = d[Name("DP")]
	}
	if paramsPresent {
		params = resolveChain(params)
	}
	switch value := params.(type) {
	case Dict:
		parameters = []Dict{value}
	case Array:
		valid := len(value) == len(filters)
		for _, item := range value {
			item = resolveChain(item)
			if dict, ok := item.(Dict); ok {
				parameters = append(parameters, dict)
			} else if _, ok := item.(Null); ok {
				parameters = append(parameters, nil)
			} else {
				valid = false
				break
			}
		}
		if !valid {
			filters = []string{invalidStreamFilter}
			parameters = nil
		}
	default:
		if paramsPresent {
			filters = []string{invalidStreamFilter}
			parameters = nil
		}
	}
	if paramsPresent && len(filters) == 0 {
		filters = []string{invalidStreamFilter}
	}
	return filters, parameters
}
