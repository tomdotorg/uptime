package main

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"upcheck"
)

func main() {
	app := tview.NewApplication()

	// Mutex to safely update data
	var mu sync.Mutex

	targets := []*upcheck.Target{
		{Name: "Google", Host: "google.com", Port: 80, IP: net.ParseIP("8.8.8.8"), IsAlive: true, Since: time.Now(), Attempts: 100, Failures: 2, Errors: map[string]int{"timeout": 1}},
		{Name: "Cloudflare", Host: "cloudflare.com", Port: 443, IP: net.ParseIP("1.1.1.1"), IsAlive: true, Since: time.Now(), Attempts: 200, Failures: 5, Errors: map[string]int{"connection refused": 3}},
		{Name: "Router", Host: "192.168.0.1", Port: 0, IP: net.ParseIP("192.168.0.1"), IsAlive: true, Since: time.Now(), Attempts: 50, Failures: 1, Errors: map[string]int{"latency": 1}},
		{Name: "Server 1", Host: "192.168.1.100", Port: 22, IP: net.ParseIP("192.168.1.100"), IsAlive: false, Since: time.Now(), Attempts: 30, Failures: 10, Errors: map[string]int{"SSH error": 5}},
		{Name: "Server 2", Host: "192.168.1.101", Port: 22, IP: net.ParseIP("192.168.1.101"), IsAlive: true, Since: time.Now(), Attempts: 40, Failures: 0, Errors: map[string]int{}},
	}
	// Sample data organized by sections

	netInfo, err := upcheck.GetNetworkInfo()
	if err != nil {
		panic("Error getting network info")
	}
	subnetTargets, gatewayTargets, internetTargets, ok := upcheck.ClassifyTargets(targets, &netInfo)
	if !ok {
		panic("Error getting network info")
	}
	sections := make(map[string][]*upcheck.Target)
	sections["Subnet"] = subnetTargets
	sections["Gateway"] = gatewayTargets
	sections["Internet"] = internetTargets
	// Function to build a table from a section
	buildTable := func(section []*upcheck.Target) *tview.Table {
		table := tview.NewTable().SetBorders(true)
		for j, target := range section {
			table.SetCell(j, 0, tview.NewTableCell(target.Name))
			table.SetCell(j, 1, tview.NewTableCell(fmt.Sprintf("%t", target.IsAlive)))
			table.SetCell(j, 2, tview.NewTableCell(target.Since.Format("15:04:05")))
			table.SetCell(j, 3, tview.NewTableCell(strconv.Itoa(target.Attempts)))
			successRate := 100
			if target.Attempts > 0 {
				successRate = 100 - (target.Failures * 100 / target.Attempts)
			}
			table.SetCell(j, 4, tview.NewTableCell(fmt.Sprintf("%d%%", successRate)))
		}
		return table
	}

	// Main UI layout
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Table references for updating
	tables := make(map[string]*tview.Table)

	// Add sections
	for sectionName, section := range sections {
		title := tview.NewTextView().SetText(sectionName).SetTextColor(tcell.ColorYellow).SetDynamicColors(true)
		table := buildTable(section)
		tables[sectionName] = table
		flex.AddItem(title, 1, 0, false)
		flex.AddItem(table, 0, 1, true)
		flex.AddItem(tview.NewBox().SetBackgroundColor(tcell.ColorDefault), 1, 0, false) // Blank line
	}

	// Function to update sections (dummy example)
	updateSections := func() {
		mu.Lock()
		defer mu.Unlock()
		for sectionName, section := range sections {
			for i := range section {
				sections[sectionName][i].Attempts++
				if i%2 == 0 {
					sections[sectionName][i].IsAlive = !sections[sectionName][i].IsAlive
				}
			}
		}
	}

	// Periodically update data
	go func() {
		for {
			time.Sleep(1 * time.Second)
			updateSections()
			// Rebuild tables dynamically
			app.QueueUpdateDraw(func() {
				mu.Lock()
				defer mu.Unlock()
				for sectionName, table := range tables {
					table.Clear()
					section := sections[sectionName]
					for j, target := range section {
						table.SetCell(j, 0, tview.NewTableCell(target.Name))
						table.SetCell(j, 1, tview.NewTableCell(fmt.Sprintf("%t", target.IsAlive)))
						table.SetCell(j, 2, tview.NewTableCell(target.Since.Format("15:04:05")))
						table.SetCell(j, 3, tview.NewTableCell(strconv.Itoa(target.Attempts)))
						successRate := 100
						if target.Attempts > 0 {
							successRate = 100 - (target.Failures * 100 / target.Attempts)
						}
						table.SetCell(j, 4, tview.NewTableCell(fmt.Sprintf("%d%%", successRate)))
					}
				}
			})
		}
	}()

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
