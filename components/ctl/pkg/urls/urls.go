package urls

const (
	APIPrefix = "/api/rh-trex-ai/v1"
	DinosaursPath = APIPrefix + "/dinosaurs"
	FossilsPath = APIPrefix + "/fossils"
	ScientistsPath = APIPrefix + "/scientists"
)

func DinosaurPath(id string) string {
	return DinosaursPath + "/" + id
}

func FossilPath(id string) string {
	return FossilsPath + "/" + id
}

func ScientistPath(id string) string {
	return ScientistsPath + "/" + id
}

