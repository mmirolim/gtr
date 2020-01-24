package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

type Index struct {
	sync.Mutex
	// map funcs to [test files] -> [Tests]
	// TODO store entities to entities
	funcToTests map[string]map[string]map[string]bool
}

func NewIndex() *Index {
	return &Index{
		funcToTests: make(map[string]map[string]map[string]bool),
	}
}

func (ind *Index) Store(fname string, data []byte) error {
	if strings.HasSuffix(fname, "_test.go") {
		return ind.storeTestFile(fname, data)
	}
	return ind.storeSrcFile(fname, data)
}

var ErrUnsupportedType = errors.New("index unsupported type")

// TODO support other types of entities and analyze types
func (ind *Index) FindEntityCallers(fname string, ent Entity) (dic map[string][]Entity, err error) {
	if ent.typ != "func" {
		fmt.Printf("FindEntityCallers unsupported entity %+v\n", ent)
		return nil, ErrUnsupportedType
	}

	ind.Lock()
	defer ind.Unlock()

	dic = map[string][]Entity{}
	entityNamesByFile := ind.funcToTests[ent.name]
	for fname, entities := range entityNamesByFile {
		ens, ok := dic[fname]
		if !ok {
			ens = make([]Entity, 0, len(entities))
		}
		for entName := range entities {
			ens = append(ens, Entity{name: entName})
		}
		dic[fname] = ens
	}
	return
}

func (ind *Index) storeSrcFile(fname string, data []byte) error {
	// TODO implement
	return nil
}

// TODO extend to not only store test file
func (ind *Index) storeTestFile(fname string, data []byte) error {
	dic, err := getTestedFuncs(data)
	if err != nil {
		return fmt.Errorf("getTestedFuncs error %v", err)
	}
	ind.updateIndex(fname, dic)
	return nil
}

func (ind *Index) updateIndex(fname string, funcs map[string][]Entity) {
	ind.Lock()
	defer ind.Unlock()
	index := map[string]map[string]bool{}
	for testName, entities := range funcs {
		for _, entity := range entities {
			if entity.typ == "func" {
				if _, ok := index[entity.name]; ok {
					index[entity.name][testName] = true
				} else {
					index[entity.name] = map[string]bool{
						testName: true,
					}
				}
			}
		}
	}
	// replace index for test file for funcs
	for funName := range index {
		_, ok := ind.funcToTests[funName]
		if ok {
			ind.funcToTests[funName][fname] = index[funName]
		} else {
			ind.funcToTests[funName] = map[string]map[string]bool{
				fname: index[funName],
			}
		}
	}
}
