package notify

// factOf is FactOf.
func factOf(alias string, answered bool, state string, farms []GatewayFarm) GatewayFact {
	f := GatewayFact{Gateway: alias}
	if !answered || state != StateFarmBroken {
		return f
	}
	f.Down = true
	for _, fm := range farms {
		if !fm.Up {
			f.Farm, f.Since = fm.Name, fm.Since
			break
		}
	}
	return f
}
