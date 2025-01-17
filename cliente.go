/*
ESTE ES EL ÚNICO ARCHIVO QUE SE PUEDE MODIFICAR

RECOMENDACIÓN: Solo modicar a partir de la parte
				donde se encuentran la explicación
				de las otras variables.

*/

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Estructuras principales
type Plane struct {
	id       int
	numPass  int
	category string
	arrival  time.Time
}

type Runway struct {
	id int
}

type Gate struct {
	id int
}

type PriorityQueue []Plane

// Implementación de la interfaz sort.Interface para PriorityQueue
func (pq PriorityQueue) Len() int { return len(pq) }
func (pq PriorityQueue) Less(i, j int) bool {
	priority := map[string]int{"A": 1, "B": 2, "C": 3}
	if pq[i].category != pq[j].category {
		return priority[pq[i].category] < priority[pq[j].category]
	}
	return pq[i].arrival.Before(pq[j].arrival)
}
func (pq PriorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }

// Estructura del Aeropuerto
type Airport struct {
	mu             sync.Mutex
	runways        chan Runway
	gates          chan Gate
	queue          PriorityQueue
	priority       string
	state          int
	maxQueueCap    int
	activeLandings int
	activeGates    int
	totalPlanes    int
	landedPlanes   int
	done           chan bool
}

// Variables globales
var (
	buf     bytes.Buffer
	logger  = log.New(&buf, "logger: ", log.Lshortfile)
	wg      sync.WaitGroup
	runways = 3
	gates   = 10
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Configuración inicial de aviones
	numA, numB, numC := 20, 5, 5
	totalPlanes := numA + numB + numC

	// Establecer conexión TCP
	conn, err := net.Dial("tcp", "localhost:8000")
	if err != nil {
		logger.Fatal(err)
	}
	defer conn.Close()

	// Inicializar el aeropuerto
	airport := &Airport{
		runways:     make(chan Runway, runways),
		gates:       make(chan Gate, gates),
		queue:       PriorityQueue{},
		priority:    "",
		state:       0,
		maxQueueCap: 50,
		done:        make(chan bool),
		totalPlanes: totalPlanes,
	}

	// Inicializar pistas
	for i := 1; i <= runways; i++ {
		airport.runways <- Runway{i}
	}

	// Inicializar puertas
	for i := 1; i <= gates; i++ {
		airport.gates <- Gate{i}
	}

	// Iniciar goroutines
	go generateInitialAirplanes(airport, numA, numB, numC)
	go handleMessages(conn, airport)
	go manageRunways(airport)
	go monitorCompletion(airport)

	// Esperar a que termine la simulación
	wg.Add(1)
	<-airport.done
	wg.Done()
	wg.Wait()
}

// Genera y encola aviones iniciales
func generateInitialAirplanes(airport *Airport, numA, numB, numC int) {
	id := 1
	planes := make([]Plane, 0, numA+numB+numC)

	// Generar aviones de cada categoría
	for i := 0; i < numA; i++ {
		planes = append(planes, Plane{
			id:       id,
			category: "A",
			numPass:  assignPassengers("A"),
			arrival:  time.Now(),
		})
		id++
	}
	for i := 0; i < numB; i++ {
		planes = append(planes, Plane{
			id:       id,
			category: "B",
			numPass:  assignPassengers("B"),
			arrival:  time.Now(),
		})
		id++
	}
	for i := 0; i < numC; i++ {
		planes = append(planes, Plane{
			id:       id,
			category: "C",
			numPass:  assignPassengers("C"),
			arrival:  time.Now(),
		})
		id++
	}

	// Mezclar aleatoriamente la lista de aviones
	rand.Shuffle(len(planes), func(i, j int) {
		planes[i], planes[j] = planes[j], planes[i]
	})

	// Encolar los aviones
	for _, plane := range planes {
		airport.enqueuePlane(plane)
	}
}

// Encola un avión en la cola de aterrizajes
func (airport *Airport) enqueuePlane(plane Plane) bool {
	airport.mu.Lock()
	defer airport.mu.Unlock()

	if len(airport.queue) >= airport.maxQueueCap {
		fmt.Printf("Cola para Categoría %s llena. Avión %d rechazado.\n", plane.category, plane.id)
		return false
	}

	airport.queue = append(airport.queue, plane)
	fmt.Printf("Avión %d de Categoría %s en cola para aterrizar.\n", plane.id, plane.category)
	return true
}

// Maneja los mensajes entrantes desde la conexión TCP
func handleMessages(conn net.Conn, airport *Airport) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		msg := strings.TrimSpace(scanner.Text())
		processMessage(msg, airport)
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Connection error:", err)
	}

	airport.mu.Lock()
	airport.state = 9
	airport.priority = ""
	airport.mu.Unlock()
}

// Procesa un mensaje recibido y actualiza el estado del aeropuerto
func processMessage(msg string, airport *Airport) {
	code, err := strconv.Atoi(msg)
	if err != nil {
		logger.Println("Invalid message:", msg)
		return
	}

	airport.mu.Lock()
	defer airport.mu.Unlock()

	switch code {
	case 0:
		airport.state = 0
		airport.priority = ""
		fmt.Println("\nAeropuerto Inactivo: No hay Aterrizajes")
	case 1:
		airport.state = 1
		airport.priority = "A"
		fmt.Println("\nSolo Categoría A puede aterrizar")
	case 2:
		airport.state = 2
		airport.priority = "B"
		fmt.Println("\nSolo Categoría B puede aterrizar")
	case 3:
		airport.state = 3
		airport.priority = "C"
		fmt.Println("\nSolo Categoría C puede aterrizar")
	case 4:
		airport.state = 4
		airport.priority = "A"
		fmt.Println("\nPrioridad de aterrizaje para Categoría A")
	case 5:
		airport.state = 5
		airport.priority = "B"
		fmt.Println("\nPrioridad de aterrizaje para Categoría B")
	case 6:
		airport.state = 6
		airport.priority = "C"
		fmt.Println("\nPrioridad de aterrizaje para Categoría C")
	case 7, 8:
		fmt.Println("\nEstado no definido: Se mantiene el estado anterior")
	case 9:
		airport.state = 9
		airport.priority = ""
		fmt.Println("\nAeropuerto Cerrado Temporalmente")
	default:
		fmt.Println("Código no reconocido")
	}
}

// Gestiona el aterrizaje de aviones según la prioridad
func manageRunways(airport *Airport) {
	for {
		airport.mu.Lock()
		if airport.state == 0 || airport.state == 9 {
			airport.mu.Unlock()
			time.Sleep(1 * time.Second)
			continue
		}

		sort.Sort(airport.queue)

		var plane *Plane
		var index int

		switch airport.state {
		case 1, 2, 3:
			desiredCategory := airport.priority
			for i, p := range airport.queue {
				if p.category == desiredCategory {
					plane = &p
					index = i
					break
				}
			}
		case 4, 5, 6:
			desiredCategory := airport.priority
			found := false
			for i, p := range airport.queue {
				if p.category == desiredCategory {
					plane = &p
					index = i
					found = true
					break
				}
			}
			if !found && len(airport.queue) > 0 {
				plane = &airport.queue[0]
				index = 0
			}
		default:
			if len(airport.queue) > 0 {
				plane = &airport.queue[0]
				index = 0
			}
		}

		if plane != nil && len(airport.runways) > 0 {
			canLand := false
			switch airport.state {
			case 1, 2, 3:
				if plane.category == airport.priority {
					canLand = true
				}
			case 4, 5, 6:
				if plane.category == airport.priority {
					canLand = true
				} else {
					priorityWaiting := false
					for _, p := range airport.queue {
						if p.category == airport.priority {
							priorityWaiting = true
							break
						}
					}
					if !priorityWaiting {
						canLand = true
					}
				}
			default:
				canLand = true
			}

			if canLand {
				airport.queue = append(airport.queue[:index], airport.queue[index+1:]...)
				airport.activeLandings++
				runway := <-airport.runways
				airport.mu.Unlock()
				go handleLanding(airport, *plane, runway)
			} else {
				airport.mu.Unlock()
				time.Sleep(500 * time.Millisecond)
			}
		} else {
			airport.mu.Unlock()
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// Maneja el aterrizaje de un avión
func handleLanding(airport *Airport, plane Plane, runway Runway) {
	fmt.Printf("Avión %d de Categoría %s está aterrizando en la Pista %d.\n", plane.id, plane.category, runway.id)
	time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
	fmt.Printf("Avión %d aterrizado y desplazándose a la puerta.\n", plane.id)

	select {
	case gate := <-airport.gates:
		fmt.Printf("Gate %d: Avión %d asignado a la puerta.\n", gate.id, plane.id)
		go handleGate(airport, plane, gate)
	default:
		fmt.Printf("No hay puertas disponibles para el Avión %d. Esperando...\n", plane.id)
		time.Sleep(1 * time.Second)
		airport.mu.Lock()
		airport.queue = append([]Plane{plane}, airport.queue...)
		airport.mu.Unlock()
	}

	airport.runways <- runway

	airport.mu.Lock()
	airport.activeLandings--
	airport.landedPlanes++
	airport.mu.Unlock()
}

// Maneja el desembarque en una puerta específica
func handleGate(airport *Airport, plane Plane, gate Gate) {
	fmt.Printf("Gate %d: Avión %d iniciando desembarque.\n", gate.id, plane.id)
	time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
	fmt.Printf("Gate %d: Avión %d ha terminado el desembarque.\n", gate.id, plane.id)

	airport.mu.Lock()
	defer airport.mu.Unlock()
	airport.activeGates--
	airport.gates <- gate
}

// Monitorea la finalización de la simulación
func monitorCompletion(airport *Airport) {
	for {
		airport.mu.Lock()
		completed := airport.landedPlanes >= airport.totalPlanes &&
			len(airport.queue) == 0 &&
			airport.activeLandings == 0 &&
			airport.activeGates == 0
		airport.mu.Unlock()

		if completed && airport.state != 9 {
			airport.done <- true
			return
		}
		time.Sleep(1 * time.Second)
	}
}

// Asigna pasajeros basados en la categoría del avión
func assignPassengers(category string) int {
	switch category {
	case "A":
		return rand.Intn(100) + 101
	case "B":
		return rand.Intn(51) + 50
	case "C":
		return rand.Intn(50)
	default:
		return 0
	}
}
