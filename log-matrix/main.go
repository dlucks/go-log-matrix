package main

import (
	"fmt"
	"os"
	"log"
	"bufio"
	"strings"
	"strconv"
	"time"
	"html/template"
)

const CONFIG_INPUT = "input"
const CONFIG_DEPTH = "depth"
const CONFIG_FROM  = "from"
const CONFIG_TO	   = "to"

/**
 * Main.
 */
func main() {

	fmt.Printf("apachelog started...\n")

	// Initialize some stuff.
	var inputFile string
	var depth int

	arguments := getArgs()

	// Get input file.
	if value, ok := arguments[CONFIG_INPUT]; ok {
		inputFile = value
	} else {
		log.Fatal("Error: No input file given")
	}

	// Get depth. Load default value if no value is set by CLI.
	if value, ok := arguments[CONFIG_DEPTH]; ok {
		value, _ := strconv.ParseInt(value, 0, 0)
		depth = int(value)
	} else {
		depth = -1
	}

	var timePattern string
	var fromTime time.Time
	var toTime time.Time

	timePattern = "20060102150405"
	fromTime = time.Now() // TODO: Set Unix Timestamp 0 as default value.
	toTime = time.Now()

	if value, ok := arguments[CONFIG_FROM]; ok {
		value, _ := time.Parse(timePattern, value)
		fromTime = value
	}

	if value, ok := arguments[CONFIG_TO]; ok {
		value, _ := time.Parse(timePattern, value)
		toTime = value
	}

	// Prepare items.
	root := Node{"/", 0, 0, make([]int64, 0), make([]*Node, 0)}
	logItems, err := getLogItems(inputFile, fromTime.Unix(), toTime.Unix())

	// Fail if error on reading log items.
	if err != nil {
		log.Fatalf("getLogItems: %s", err)
	}

	// Add all log items to tree structure.
	for _, logItem := range logItems {
		root.Add(logItem.Url, logItem.UnixTimestamp)
	}

	firstLogItemTimestamp := root.GetFirstCallTimestamp()
	lastLogItemTimestamp  := root.GetLastCallTimestamp()

	// Generate some output.
	fmt.Println("Input file: ", inputFile)
	fmt.Println("Depth: ", depth)
	fmt.Println("From time: ", fromTime)
	fmt.Println("To time: ", toTime)
	fmt.Println("Lines found: ", len(logItems))
	fmt.Println("Oldest: ", firstLogItemTimestamp)
	fmt.Println("Newest: ", lastLogItemTimestamp)

	print(&root, depth);

	nList := flattenTree(&root, depth)
	for _, child := range nList {
		fmt.Println(child.Url, child.Count)
	}

	tmpl, err := template.ParseFiles("test.html")

	if err != nil {
		log.Fatal(err)
	}

	templateData := TemplateData{
		nList,
		fromTime,
		toTime,
	}

	// Write HTML template to output file.
	f, _ := os.Create("output.html")
	tmpl.Execute(f, templateData)

}

// Just for debugging.
func print(node *Node, maxLevel int) {

	if maxLevel > -1 && node.Level > maxLevel {
		return
	}

	fmt.Println(strings.Repeat("-", node.Level), node.Url, "(" + strconv.Itoa(node.Count) + ")")

	if node.HasChildren() {
		for _, child := range node.Children {
			print(child, maxLevel)
		}
	}
}

/**
 * Flatten tree structured nodes into flatten node list.
 */
func flattenTree(node *Node, maxLevel int) []*Node {

	if maxLevel > -1 && node.Level > maxLevel {
		return nil
	}

	newList := []*Node{node}

	if node.HasChildren() {

		for _, child := range node.Children {
			newList = append(newList, flattenTree(child, maxLevel)...)
		}

	}

	return newList
}

/**
 * Get configuration by CLI arguments.
 */
func getArgs() map[string]string {

	// Get CLI arguments as array.
	cliArgs := os.Args
	cliArgsLength := len(cliArgs)

	// Initialize empty map.
	args := make(map[string]string)

	// Fill configuration map with information from CLI arguments.
	for i := 0; i < cliArgsLength - 1; i++ {
		switch cliArgs[i] {
		case "-i": args[CONFIG_INPUT] = cliArgs[i + 1]
		case "-d": args[CONFIG_DEPTH] = cliArgs[i + 1]
		case "-f": args[CONFIG_FROM]  = cliArgs[i + 1]
		case "-t": args[CONFIG_TO]    = cliArgs[i + 1]
		}
	}

	return args
}

/**
 * Read file.
 */
func getLogItems(path string, fromTime int64, toTime int64) ([]LogItem, error) {

	// Open given file.
	file, err := os.Open(path)

	// Error while opening file.
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var lines []LogItem
	scanner := bufio.NewScanner(file)

	var logItemTimePattern string
	logItemTimePattern = "[02/Jan/2006:15:04:05"

	// Iterate over each line.
	for scanner.Scan() {

		// Extract line items.
		line := scanner.Text()
		terms := strings.Split(line, " ")

		// Ensure there is enogh data in line.
		if (len(terms) < 8) {
			continue
		}

		// Get URL (without paramsters).
		urlTerm := terms[6]
		url := urlTerm // TODO: Strip GET parameters.

		// Get timestamp.
		timestampTerm := terms[3]
		timestamp, _ := time.Parse(logItemTimePattern, timestampTerm)
		timestampUnix := timestamp.Unix()

		if (timestampUnix >= fromTime && timestampUnix <= toTime) {
			lines = append(lines, LogItem{url, timestampUnix})
		}
	}

	return lines, scanner.Err()
}

/**
 * Data Structure: Template Data.
 */
type TemplateData struct {
	NodeList []*Node
	FromTime time.Time
	ToTime time.Time
}

/**
 * Data Structure: Log Item.
 */
type LogItem struct {
	Url           string
	UnixTimestamp int64
}

/**
 * Data Structure: Node.
 */
type Node struct {
	Url            string
	Count          int
	Level          int
	CallTimestamps []int64
	Children       []*Node
}

func (n *Node) Add(calledUrl string, timestamp int64) {

	n.Count++;
	n.CallTimestamps = append(n.CallTimestamps, timestamp)

	terms := strings.Split(calledUrl, "/")

	// Propagate to child nodes.
	if (len(terms) > n.Level + 1) {
		child := n.GetChildByUrl(calledUrl)
		child.Add(calledUrl, timestamp)
	}
}

/**
 * Check if node has child nodes.
 */
func (n *Node) HasChildren() bool {
	return (n.Children != nil && len(n.Children) > 0)
}

/**
 * Create a new child node.
 */
func (n *Node) CreateChild(calledUrl string) *Node {

	terms := strings.Split(calledUrl, "/")
	level := n.Level + 1
	terms = terms[0 : level + 1]
	url := strings.Join(terms, "/")

	child := Node{url, 0, level, make([]int64, 0), nil}

	if (n.Children == nil) {
		n.Children = make([]*Node, 0)
	}

	n.Children = append(n.Children, &child)

	return &child
}

func (n *Node) GetFirstCallTimestamp() int64 {
	if (len(n.CallTimestamps) == 0) {
		return 0
	}
	return ArrayMaxInt64(n.CallTimestamps)
}

func (n *Node) GetLastCallTimestamp() int64 {
	if (len(n.CallTimestamps) == 0) {
		return 0
	}
	return ArrayMinInt64(n.CallTimestamps)
}

func (n *Node) GetChildByUrl(calledUrl string) *Node {

	// Append a new child node if no child exists at all.
	if n.HasChildren() == false {
		return n.CreateChild(calledUrl)
	}

	for _, child := range n.Children {
		if strings.HasPrefix(calledUrl, child.Url) {
			return child
		}
	}

	// Obviously no matching child has been found. Create a new one.
	return n.CreateChild(calledUrl)
}

func ArrayMinInt64(values []int64) int64 {
	var min int64
	for key, value := range values {
		if key == 0 || value < min {
			min = value
		}
	}
	return min
}

func ArrayMaxInt64(values []int64) int64 {
	var max int64
	for key, value := range values {
		if key == 0 || value > max {
			max = value
		}
	}
	return max
}