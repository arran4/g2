package g2


func (a *AggUseFlag) Desc() string {
	if a.GlobalDesc != "" {
		return a.GlobalDesc
	}
	descCounts := make(map[string]int)
	for _, desc := range a.LocalDescs {
		if desc != "" {
			descCounts[desc]++
		}
	}
	for _, desc := range a.MetadataDescs {
		if desc != "" {
			descCounts[desc]++
		}
	}
	maxCount := 0
	maxDesc := ""
	for desc, count := range descCounts {
		if count > maxCount {
			maxCount = count
			maxDesc = desc
		} else if count == maxCount && desc < maxDesc { // break ties for deterministic output
			maxDesc = desc
		}
	}
	return maxDesc
}
