package main

func Having(all []string, val string) bool {
	for _, v := range all {
		if v == val {
			return true
		}
	}
	return false
}
