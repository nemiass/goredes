package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	ErrorFileJsonNotFound = errors.New("Error no existe `config.json`")
	ErrorJsonStruct       = errors.New("Error con los datos de el `config.json`")
)

type Color string

const (
	ColorBlack  Color = "\u001b[30m"
	ColorRed          = "\u001b[31m"
	ColorGreen        = "\u001b[32m"
	ColorYellow       = "\u001b[33m"
	ColorBlue         = "\u001b[34m"
	ColorReset        = "\u001b[0m"
)

// funcion para limpiar la pantalla
func clear() {
	sys := runtime.GOOS
	if sys == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

func pop(list_routers []*Router) ([]*Router, *Router, error) {
	if len(list_routers) == 0 {
		err := fmt.Errorf("Error slice: no pop slice empty")
		return list_routers, &Router{}, err
	}
	tmp := list_routers[0]
	return list_routers[1:], tmp, nil
}

func remove(list_routers []*Router, indice int) []*Router {
	return append(list_routers[:indice], list_routers[indice+1:]...)
}

//variable para serializar los datos del json con json.Unmarshal()
type JsonData struct {
	ConfGenerales struct {
		IntLanDefault string
		IntWanDefault string
	}
	Routers []struct {
		IdRouter string
		Area     string
		Lans     []struct {
			Int    string
			Puerto string
			IPRed  string
		}
		Wans []struct {
			Int          string
			Vecino       string
			Puerto       string
			PuertoVecino string
			IPRed        string
			Dce          string
			DceVecino    string
		}
	}
}

type Red struct {
	Interface string
	Port      string
	Ip        string
	Mask      string
	IpRed     []string
	Dce       string
	Wildcard  string
}

func (r *Red) addIPLan(ipRed string) (err error) {
	if ipRed == "" {
		return ErrorJsonStruct
	}
	ipMask := strings.Split(strings.Replace(ipRed, "/", ",", 1), ",")
	r.Ip = r.resolveIp(ipMask[0])
	maskInt, err := strconv.Atoi(ipMask[1])
	r.Mask = r.resolveMask(maskInt)
	r.IpRed = ipMask
	r.Wildcard = r.resolveWildcard(r.Mask)
	return nil
}

func (r Red) resolveWildcard(mask string) string {
	mask_split := strings.Split(mask, ".")
	wildcard := make([]string, 0)

	for _, m := range mask_split {
		nm, _ := strconv.Atoi(m)
		wildcard = append(wildcard, strconv.Itoa(255-nm))
	}

	return strings.Join(wildcard, ".")
}

func (r *Red) resolveIp(ip string) string {
	ip_temp := strings.Split(ip, ".")
	new_octeto, _ := strconv.Atoi(ip_temp[len(ip_temp)-1])
	ip_temp[len(ip_temp)-1] = strconv.Itoa(new_octeto + 1)
	return strings.Join(ip_temp, ".")
}

func (r Red) resolveMask(mask int) string {
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

func (r *Red) addIpWan(ipRed string) (err error) {
	ip_and_mask := strings.Split(strings.Replace(ipRed, "/", ",", 1), ",")
	ip_temp := strings.Split(ip_and_mask[0], ".")
	new_octeto, _ := strconv.Atoi(ip_temp[len(ip_temp)-1])

	if r.Dce == "true" {
		ip_temp[len(ip_temp)-1] = strconv.Itoa(new_octeto + 1)
	} else {
		ip_temp[len(ip_temp)-1] = strconv.Itoa(new_octeto + 2)
	}

	r.Ip = strings.Join(ip_temp, ".")
	mask_int, err := strconv.Atoi(ip_and_mask[1])
	r.Mask = r.resolveMask(mask_int)
	r.IpRed = ip_and_mask
	r.Wildcard = r.resolveWildcard(r.Mask)
	return nil
}

type Router struct {
	Id      string
	Area    string
	Vecinos map[string]Red
	Lans    []Red
	Wans    []Red
}

func (rv *Router) addVecino(id_vecino string, red Red) {
	rv.Vecinos[id_vecino] = red
}

type Topology struct {
	Routers          map[string]*Router
	IntLanDefault    string
	IntWanDefault    string
	RoutersVisitados []*Router
	FilePath         string
}

func (t *Topology) static() error {

	var main_vecinos []*Router
	routing_static := make(map[string][]*Router)
	// Algoritmo para encontrar todos con los que un router se conecta, para hacer el static routing
	for _, router := range t.Routers {
		for rvecinoId := range router.Vecinos {
			main_vecinos = append(main_vecinos, t.Routers[rvecinoId])
		}

		var cola []*Router

		for _, main_vecino := range main_vecinos {
			for vecino_cola := range main_vecino.Vecinos {
				if t.Routers[vecino_cola].Id != router.Id {
					cola = t.add_cola(cola, t.Routers[vecino_cola])
				}

				var rout_v *Router
				var err error

				for len(cola) > 0 {
					cola, rout_v, err = pop(cola)

					if err != nil {
						showError(err)
						os.Exit(0)
					}

					for new_cola := range rout_v.Vecinos {
						if t.Routers[new_cola].Id != router.Id {
							cola = t.add_cola(cola, t.Routers[new_cola])
						}
					}
				}
			}
		}
		routing_static[router.Id] = append(routing_static[router.Id], t.RoutersVisitados...)
		t.RoutersVisitados = t.RoutersVisitados[:0]
	}

	// Agregando enrrutamiento estático a los .txt
	for key_router, routers := range routing_static {
		router_txt := "router_" + key_router + ".txt"
		content, err := ioutil.ReadFile(t.FilePath + "/_routers_/" + router_txt)

		if err != nil {
			return fmt.Errorf(ColorRed+"Error al leer `%s`"+ColorReset, router_txt)
		}

		routing_text := ""
		for _, router_conect := range routers {

			for _, lan := range router_conect.Lans {
				routing_text += "ip route " + lan.IpRed[0] + " " + lan.Mask + " "
				routing_text += t.Routers[key_router].Vecinos[router_conect.Id].Ip + "\n"
			}
		}

		content = append(content, []byte(routing_text)...)
		err = ioutil.WriteFile(t.FilePath+"/_routers_/"+router_txt, content, 755)

		if err != nil {
			return fmt.Errorf(ColorRed+"Error al escribir `%s`"+ColorReset, router_txt)
		}
	}
	return nil
}

func (t *Topology) add_cola(cola []*Router, router *Router) []*Router {
	for _, queue := range cola {
		if queue.Id == router.Id {
			return cola
		}
	}

	for _, camino := range t.RoutersVisitados {
		if camino.Id == router.Id {
			return cola
		}
	}
	t.RoutersVisitados = append(t.RoutersVisitados, router)
	return append(cola, router)
}

func (t Topology) writeFile(file_name string, content []byte) error {
	err := ioutil.WriteFile(t.FilePath+"/_routers_/"+file_name, content, 755)
	if err != nil {
		return err
	}
	return nil
}

func (t *Topology) ripV1() error {
	for key_router, router := range t.Routers {
		file_name := fmt.Sprintf("router_%s.txt", key_router)
		content, err := ioutil.ReadFile(t.FilePath + "/_routers_/" + file_name)

		if err != nil {
			return fmt.Errorf(ColorRed+"Error al leer `%s`"+ColorReset, file_name)
		}

		new_content := "route rip\nversion 1\n"
		for _, lan := range router.Lans {
			new_content += "nerwork " + lan.IpRed[0] + "\n"
		}

		for _, wan := range router.Wans {
			new_content += "nerwork " + wan.IpRed[0] + "\n"
		}

		content = append(content, []byte(new_content)...)

		err = t.writeFile(file_name, content)

		if err != nil {
			return fmt.Errorf(ColorRed+"Error al escribir `%s`"+ColorReset, file_name)
		}
	}
	return nil
}

func (t *Topology) ospf() error {
	for key_router, router := range t.Routers {
		file_name := fmt.Sprintf("router_%s.txt", key_router)
		content, err := ioutil.ReadFile(t.FilePath + "/_routers_/" + file_name)

		if err != nil {
			return fmt.Errorf(ColorRed+"Error al leer `%s`"+ColorReset, file_name)
		}

		new_content := "router ospf 1\n"
		for _, lan := range router.Lans {
			new_content += "network " + lan.IpRed[0] + " "
			new_content += lan.Wildcard + " area " + router.Area + "\n"
		}

		for _, wan := range router.Wans {
			new_content += "network " + wan.IpRed[0] + " "
			new_content += wan.Wildcard + " area " + router.Area + "\n"
		}

		content = append(content, []byte(new_content)...)

		err = t.writeFile(file_name, content)

		if err != nil {
			return fmt.Errorf(ColorRed+"Error al escribir `%s`"+ColorReset, file_name)
		}
	}
	return nil
}

func (t *Topology) eigrp() error {
	for key_router, router := range t.Routers {
		file_name := fmt.Sprintf("router_%s.txt", key_router)
		content, err := ioutil.ReadFile(t.FilePath + "/_routers_/" + file_name)

		if err != nil {
			return fmt.Errorf(ColorRed+"Error al leer `%s`"+ColorReset, file_name)
		}

		new_content := "router eigrp 1\n"
		for _, lan := range router.Lans {
			new_content += "network " + lan.IpRed[0] + " "
			new_content += lan.Wildcard + "\n"
		}

		for _, wan := range router.Wans {
			new_content += "network " + wan.IpRed[0] + " "
			new_content += wan.Wildcard + "\n"
		}

		content = append(content, []byte(new_content)...)

		err = t.writeFile(file_name, content)

		if err != nil {
			return fmt.Errorf(ColorRed+"Error al escribir `%s`"+ColorReset, file_name)
		}
	}
	return nil
}

func (t *Topology) bgp() error {
	for key_router, router := range t.Routers {
		file_name := fmt.Sprintf("router_%s.txt", key_router)
		content, err := ioutil.ReadFile(t.FilePath + "/_routers_/" + file_name)

		if err != nil {
			return fmt.Errorf(ColorRed+"Error al leer `%s`"+ColorReset, file_name)
		}

		new_content := "router bgp " + router.Area + "\n"
		for _, lan := range router.Lans {
			new_content += "network " + lan.IpRed[0] + " "
			new_content += "mask " + lan.Mask + "\n"
		}

		for _, wan := range router.Wans {
			new_content += "network " + wan.IpRed[0] + " "
			new_content += "mask " + wan.Mask + "\n"
		}

		for key_vecino, red_vecino := range router.Vecinos {
			new_content += "neighbor " + red_vecino.Ip + " "
			new_content += "remote-as " + t.Routers[key_vecino].Area + "\n"
		}

		content = append(content, []byte(new_content)...)

		err = t.writeFile(file_name, content)

		if err != nil {
			return fmt.Errorf(ColorRed+"Error al escribir `%s`"+ColorReset, file_name)
		}
	}
	return nil
}

type GoRedes struct {
	FilePath string
	Topology
}

func (g GoRedes) startConfig() (err error) {
	menu := `
-------------------------------
  Inicializando 'config.json'
`
	fmt.Println(ColorGreen, menu, ColorReset)
	var numRouters string

	for {

		fmt.Print(ColorGreen, "numero de routers #> ", ColorYellow)
		fmt.Scan(&numRouters)

		if num, err := strconv.Atoi(numRouters); err == nil {

			if num <= 26 {

				// comprobando existencia de `config.json`
				if _, err := os.Stat(g.FilePath + "/config.json"); !os.IsNotExist(err) {
					fmt.Print(ColorGreen, "*se reemplazará `config.json` [s/n] #> ", ColorYellow)
					var new_config string
					fmt.Scan(&new_config)
					if new_config != "s" {
						fmt.Println("`config.json` no fue modificado")
						return nil
					}
				}

				indices := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

				json_init := JsonData{}
				json_init.ConfGenerales.IntLanDefault = "gigabitEthernet"
				json_init.ConfGenerales.IntWanDefault = "serial"

				// al usar maps apra crear los json, da problemas, ya que los maps internamente se ordenan
				for i := 0; i < num; i++ {
					json_init.Routers = append(json_init.Routers, struct {
						IdRouter string
						Area     string
						Lans     []struct {
							Int    string
							Puerto string
							IPRed  string
						}
						Wans []struct {
							Int          string
							Vecino       string
							Puerto       string
							PuertoVecino string
							IPRed        string
							Dce          string
							DceVecino    string
						}
					}{
						IdRouter: string(indices[i]),
						Area:     "0",
						Lans: []struct {
							Int    string
							Puerto string
							IPRed  string
						}{
							{Int: "default", Puerto: "", IPRed: ""},
						},
						Wans: []struct {
							Int          string
							Vecino       string
							Puerto       string
							PuertoVecino string
							IPRed        string
							Dce          string
							DceVecino    string
						}{
							{Int: "default", Vecino: "", Puerto: "", PuertoVecino: "", IPRed: "", Dce: "false", DceVecino: "false"},
						},
					})
				}

				file, err := json.MarshalIndent(json_init, "", "   ")

				if err != nil {
					// retornando errores, los podemos mostrar coon a funcion showErrors()
					return fmt.Errorf("Error al codificar `config.json`!!!")
				} else {
					log.Print("`config.json` codificado")
				}

				locFile := g.FilePath + "/config.json"
				err = ioutil.WriteFile(locFile, file, 0644)

				if err != nil {
					return fmt.Errorf("Error al guardar el archivo `config.json`")
				} else {
					log.Print("`config.json` generado")
				}
				return nil
			}
		}
	}
}

func (g *GoRedes) loadDataJson() (err error) {
	// comprobando si existe o no el archivo y creandolo
	if _, err := os.Stat(g.FilePath + "/_routers_"); err != nil {
		if os.IsNotExist(err) {
			os.Mkdir(g.FilePath+"/_routers_", 0755)
			log.Print("directorio `_routers_` creado")
		}
	}

	file, err := ioutil.ReadFile(g.FilePath + "/config.json")
	if err != nil {
		return ErrorFileJsonNotFound
	}

	json_data := JsonData{}

	err = json.Unmarshal(file, &json_data)
	if err != nil {
		return fmt.Errorf("Error al decodificar `config.json`")
	}

	g.Topology.IntLanDefault = json_data.ConfGenerales.IntLanDefault
	g.Topology.IntWanDefault = json_data.ConfGenerales.IntWanDefault
	g.Topology.Routers = make(map[string]*Router)

	var router *Router
	var routerVecino *Router

	for _, rJson := range json_data.Routers {
		if _, ok := g.Topology.Routers[rJson.IdRouter]; ok {
			router = g.Topology.Routers[rJson.IdRouter]
		} else {
			router = &Router{}
			router.Id = rJson.IdRouter
			router.Vecinos = make(map[string]Red)
		}
		router.Area = rJson.Area

		// agregando interfaces al router
		for _, l := range rJson.Lans {
			if l.IPRed == "" || l.Puerto == "" {
				continue
			}
			red_lan := Red{}
			if l.Int == "default" {
				red_lan.Interface = g.IntLanDefault
			} else {
				red_lan.Interface = l.Int
			}
			red_lan.Port = l.Puerto
			red_lan.addIPLan(l.IPRed)
			router.Lans = append(router.Lans, red_lan)
		}

		for _, w := range rJson.Wans {
			if w.IPRed == "" || w.Puerto == "" || w.PuertoVecino == "" || w.Vecino == "" {
				continue
			}
			red_wan := Red{}
			if w.Int == "default" {
				red_wan.Interface = g.IntWanDefault
			} else {
				red_wan.Interface = w.Int
			}
			red_wan.Dce = w.Dce
			red_wan.Port = w.Puerto
			red_wan.addIpWan(w.IPRed)
			router.Wans = append(router.Wans, red_wan)

			if _, okk := g.Topology.Routers[w.Vecino]; okk {
				routerVecino = g.Topology.Routers[w.Vecino]
			} else {
				routerVecino = &Router{}
				routerVecino.Id = w.Vecino
				routerVecino.Vecinos = make(map[string]Red)
			}

			red_wan_vecino := Red{}
			red_wan_vecino.Interface = red_wan.Interface
			red_wan_vecino.Dce = w.DceVecino
			red_wan_vecino.Port = w.PuertoVecino
			red_wan_vecino.addIpWan(w.IPRed)
			routerVecino.Wans = append(routerVecino.Wans, red_wan_vecino)

			router.addVecino(routerVecino.Id, red_wan_vecino)
			routerVecino.addVecino(router.Id, red_wan)
			g.Topology.Routers[router.Id] = router
			g.Topology.Routers[routerVecino.Id] = routerVecino
		}
	}
	return nil
}

func (g GoRedes) startRouting() {
	menu_routing := `
-------------------------------------
    > Enrrutamiento <
[1] estatico  <off>
[2] rip v1     on
[3] ospf       on
[4] eigrp     <off>
[5] bgp       <off> 
[6] salir     <off>
`
	fmt.Println(ColorGreen, menu_routing, ColorReset)
	var option_r string
	fmt.Print(ColorGreen, "#> ", ColorYellow)
	fmt.Scan(&option_r)

	switch option_r {
	case "1":
		g.Topology.static()
		fmt.Println(ColorYellow + ">> Enrrutamiento `estatic` completo <<" + ColorReset)
		fmt.Println(ColorRed + "OFF: apagado desde el codigo fuente" + ColorReset)
	case "2":
		g.Topology.ripV1()
		fmt.Println(ColorYellow + ">> Enrrutamiento `rip v1` completo <<" + ColorReset)
	case "3":
		g.Topology.ospf()
		fmt.Println(ColorYellow + ">> Enrrutamiento `ospf` completo <<" + ColorReset)
	case "4":
		fmt.Println(ColorRed + "OFF: apagado desde el codigo fuente" + ColorReset)
		g.Topology.eigrp()
		fmt.Println(ColorYellow + ">> Enrrutamiento `eigrp` completo <<" + ColorReset)
	case "5":
		fmt.Println(ColorRed + "OFF: apagado desde el codigo fuente" + ColorReset)
		g.Topology.bgp()
		fmt.Println(ColorYellow + ">> Enrrutamiento `bgp` completo <<" + ColorReset)
	case "6":
		return
	}
}

func (g GoRedes) createStartFile() error {
	file_config := make(map[string][]byte)

	for key, router := range g.Topology.Routers {
		text_config := make([]byte, 0)
		line := "en\nconf t\n"
		text_config = append(text_config, []byte(line)...)

		for _, lan := range router.Lans {
			line = "int " + lan.Interface + " " + lan.Port
			line += "\nip add " + lan.Ip + " " + lan.Mask + "\nno sh\nex\n"
		}

		for _, wan := range router.Wans {
			line += "int " + wan.Interface + " " + wan.Port + "\n"
			line += "ip add " + wan.Ip + " " + wan.Mask + "\n"
			if wan.Dce == "true" {
				line += "clock rate 128000\n"
			}
			line += "no sh\nex\n"
		}

		text_config = append(text_config, line...)
		file_config[key] = text_config
		file_name := "router_" + key + ".txt"

		err := ioutil.WriteFile(g.FilePath+"/_routers_/"+file_name, text_config, 755)
		if err != nil {
			return fmt.Errorf(ColorRed + "Error de escritura de los .txt" + ColorReset)
		}
	}

	return nil
}

func showError(err error) {
	if err != nil {
		fmt.Println(ColorRed, "Error: ", err, ColorReset)
	}
}

func (g *GoRedes) aboutDev() {
	var logo string = `
     _____         _____  ___                   _      
    | __  | _ _   |   | ||_  | _____  ___  ___ |_| ___ 
    | __ -|| | |  | | | ||_  ||     || -_||_ -|| ||_ -|
    |_____||_  |  |_|___||___||_|_|_||___||___||_||___|
           |___|`
	fmt.Println(ColorBlue + logo + ColorReset)
	fmt.Println(ColorYellow + "\t* Github: https://github.com/nemiass\n")
}

func main() {

	//defer fmt.Println(ColorReset)

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(err)
	}
	logo := `
     _____   ____   _____   ______  _____   ______   _____ 
    / ____| / __ \ |  __ \ |  ____||  __ \ |  ____| / ____|
   | |  __ | |  | || |__) || |__   | |  | || |__   | (___  
   | | |_ || |  | ||  _  / |  __|  | |  | ||  __|   \___ \ 
   | |__| || |__| || | \ \ | |____ | |__| || |____  ____) |
    \_____| \____/ |_|  \_\|______||_____/ |______||_____/ 
                { - developed in GO - }`
	mainMenu := `
[1] - generar config
[2] - enrrutar
[3] - salir
[4] - about dev`

	var option string
	appRedes := GoRedes{FilePath: dir}
	appRedes.Topology.FilePath = dir

	for {
		clear()
		fmt.Println(ColorYellow, logo, ColorReset)
		fmt.Println(ColorGreen, mainMenu, ColorReset)
		fmt.Print(ColorGreen, "#> ", ColorYellow)
		fmt.Scan(&option)

		switch option {
		case "1":
			appRedes.startConfig()
		case "2":
			err = appRedes.loadDataJson()
			if err != nil {
				showError(err)
				break
			}

			err = appRedes.createStartFile()
			if err != nil {
				showError(err)
				break
			}

			appRedes.startRouting()

		case "3":
			fmt.Println(ColorGreen, "SALIENDO ...", ColorReset)
			return
		case "4":
			clear()
			appRedes.aboutDev()
		default:
			fmt.Println(ColorRed, "???", ColorReset)
		}
		fmt.Println(ColorGreen, "[ENTER] ...", ColorReset)
		fmt.Scanf("\n%s")
	}
}
