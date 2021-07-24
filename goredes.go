package main

import (
	"fmt"
	"strconv"
	"strings"
)

type Red struct {
	Interface string
	Port      string
	Ip        string
	Mask      string
	IpRed     string
	Dce       bool
}

type Router struct {
	Id      string
	Area    string
	Vecinos map[string]Router
	Lans    []Red
	Wans    []Red
}

type Topology struct {
	Routers       map[string]Router
	IntLanDefault string
	IntWanDefault string
}

func (r Topology) resolveMask(mask int) string {
	var acum string
	var list_mask []string
	for i := 1; i <= 32; i++ {
		if i <= mask {
			acum += "1"
		} else {
			acum += "0"
		}
		if i%8 == 0 {
			m_decimal, _ := strconv.ParseInt(acum, 2, 32)
			list_mask = append(list_mask, strconv.Itoa(int(m_decimal)))
			acum = ""
		}
	}
	return strings.Join(list_mask, ".")
}

type GoRedes struct {
}

func (g GoRedes) startConfig() {
	menu := `
-------------------------------
  Inicializando 'config.json'
`
	fmt.Println(menu)
	var numRouters string
	for {
		fmt.Print("numero de routers #> ")
		fmt.Scan(&numRouters)
		if num, err := strconv.Atoi(numRouters); err == nil {
			if num <= 26 {
				// TODO
				fmt.Println("Inicializar config.json")
				return
			}
		}
	}
}

func main() {
	mainMenu := `
	{GOREDES - developed in GO}
[1] - generar config
[2] - enrrutar
[3] - salir
[4] - about dev`

	var option string
	appRedes := GoRedes{}

	for {
		fmt.Println(mainMenu)
		fmt.Print("#> ")
		fmt.Scan(&option)

		switch option {
		case "1":
			appRedes.startConfig()
		case "2":
			fmt.Println("enrrutar")
		case "3":
			fmt.Println("SALIENDO ...")
			return
		case "4":
			fmt.Println("about dev")
		default:
			fmt.Println("[ENTER] continuar")
		}
	}
}
