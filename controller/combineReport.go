package controller

import (
	"context"
	"fmt"
	"go_fiber_Zoom_Report/config"
	"go_fiber_Zoom_Report/models"
	"go_fiber_Zoom_Report/utils"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Staff struct {
	EmployeeID string `bson:"employeeId" json:"employeeId"`
	Name       string `bson:"name" json:"name"`
	Branch     string `bson:"branch" json:"branch"`
	Profile    string `bson:"profile" json:"profile"`
}

type StaffReport struct {
	Name          string                    `json:"name"`
	Branch        string                    `json:"branch"`
	EmployeeID    string                    `json:"employeeId"`
	Profile       string                    `json:"profile"`
	Attendee      int                       `json:"attendee"`
	TotalAttendee int                       `json:"totalAttendees"`
	Sales         map[string]int            `json:"sales"`
	YearSale      map[string]map[string]int `json:"yearSale"`
	DilerReport   ReportBlock               `json:"dilerReport"`
	CRMReport     ReportBlock               `json:"crmReport"`
	AdvisorReport ReportBlock               `json:"advisorReport"`
	AvyuktaReport ReportBlock               `json:"avyuktaReport"`
}

type StaffDailyReport struct {
	Name          string                    `json:"name"`
	Branch        string                    `json:"branch"`
	EmployeeID    string                    `json:"employeeId"`
	Profile       string                    `json:"profile"`
	Attendee      int                       `json:"attendee"`
	TotalAttendee int                       `json:"totalAttendees"`
	Sales         map[string]int            `json:"sales"`
	DilerReport   []EveryDayReport          `json:"dilerReport"`
	YearSale      map[string]map[string]int `json:"yearSale"`
	CRMReport     []EveryDayReport          `json:"crmReport"`
	AdvisorReport []EveryDayReport          `json:"advisorReport"`
	AvyuktaReport []EveryDayReport          `json:"avyuktaReport"`
}

type EveryDayReport struct {
	Date      string `bson:"_id" json:"date"`
	TotalTime int    `bson:"totalTime" json:"totalTime"`
}

type ReportBlock struct {
	TotalCount           int         `json:"totalCount"`
	NonZeroDurationCount int         `json:"nonZeroDurationCount"`
	ZeroDurationCount    int         `json:"zeroDurationCount"`
	TotalDuration        int         `json:"totalDuration"`
	FirstCallObject      interface{} `json:"firstCallObject"`
	LastCallObject       interface{} `json:"lastCallObject"`
}

type SalesLead struct {
	L1               string    `bson:"L1"`
	L2L3             string    `bson:"L2/L3"`
	DateOfEnrollment time.Time `bson:"Date of Enrollment"`
	StudentName      string    `bson:"Student Name"`
	Source           string    `bson:"Source"`
	Year             string    `bson:"Year"`
}

type YearSales struct {
	ID   string `bson:"_id"`
	L1   int    `bson:"L1"`
	L2L3 int    `bson:"L2L3"`
}

type AdvisingController struct{}
type crmLeadsController struct{}
type ClientLeadsController struct{}
type DialerLeadsController struct{}
type Attendees struct{}

func GetCombineReport(c *fiber.Ctx) error {
	fromDateStr := c.Query("fromDate")
	toDateStr := c.Query("toDate")

	startOfDay, endOfDay, err := utils.ParseDateRange(fromDateStr, toDateStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid date format. Use YYYY-MM-DD"})
	}

	fmt.Println("🕐 Start:", startOfDay)
	fmt.Println("🕐 End:", endOfDay)

	staffCollection := config.GetCollection("ZoomDB", "staffs")
	callLogsCollection := config.GetCollection("ZoomDB", "calllogs")
	avyuktaCallCallection := config.GetCollection("ZoomDB", "avyuktacalls")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Create find options
	findOptions := options.Find().SetProjection(bson.M{
		"employeeId": 1,
		"name":       1,
		"branch":     1,
		"profile":    1,
	})

	// Filter: exclude role = "block"
	// filter := bson.M{
	// 	"role": bson.M{
	// 		"$ne": "block", // not equal
	// 	},
	// }

	filter := bson.M{
		"$and": []bson.M{
			{"role": bson.M{"$ne": "block"}},
			{"profile": bson.M{"$ne": "admin"}},
		},
	}

	cursor, err := staffCollection.Find(ctx, filter, findOptions)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch staff"})
	}

	var staffList []models.Staff
	if err := cursor.All(ctx, &staffList); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode staff"})
	}

	var finalReport []StaffReport

	// Example Loop
	// Step 3: Loop through each employee
	for _, s := range staffList {
		empID := s.EmployeeID // <-- use struct field
		name := s.Name
		branch := s.Branch
		profile := s.Profile

		var (
			advisingNumbers []string
			crmNumbers      []string
			zoomNumbers     []string
			dialerNumbers   []string
		)

		wg := sync.WaitGroup{}
		wg.Add(4)

		go func() {
			defer wg.Done()
			advisingNumbers, _ = (&AdvisingController{}).GetAdvisingNumbersByEmployeeID(empID)
		}()
		go func() {
			defer wg.Done()
			crmNumbers, _ = (&crmLeadsController{}).GetCRMLeadsNumbersByEmployeeID(empID)
		}()
		go func() {
			defer wg.Done()
			zoomNumbers, _ = (&ClientLeadsController{}).GetClientLeadsNumbersByEmployeeID(empID)
		}()

		go func() {
			defer wg.Done()
			dialerNumbers, _ = (&DialerLeadsController{}).GetDialerLeadsNumbersByEmployeeID(empID)
		}()

		// go func(){
		// 	defer wg.Done()
		// 	attendee,_ =(&Attendees{}).getAllAttendeeCount(name, startOfDay, endOfDay)
		// }()

		wg.Wait()

		// Combine both slices
		allNumbers := append(zoomNumbers, dialerNumbers...)
		// Optional: remove duplicates
		allNumbers = removeDuplicates(allNumbers)

		// ✅ Calculate each report section
		dilerReport := getCallReport(callLogsCollection, empID, allNumbers, startOfDay, endOfDay)
		crmReport := getCallReport(callLogsCollection, empID, crmNumbers, startOfDay, endOfDay)
		advisorReport := getCallReport(callLogsCollection, empID, advisingNumbers, startOfDay, endOfDay)
		avyuktaReport := getAvyuktaCallReport(avyuktaCallCallection, name, startOfDay, endOfDay)
		attendee, totalAttendees := getAttendeeCounts(name, startOfDay, endOfDay)
		sales := getSalesReport(name, startOfDay, endOfDay)
		yearSale := getSalesReportByYear(name)

		finalReport = append(finalReport, StaffReport{
			Name:          name,
			Branch:        branch,
			EmployeeID:    empID,
			Profile:       profile,
			Attendee:      attendee,
			TotalAttendee: totalAttendees,
			Sales:         sales,
			YearSale:      yearSale,
			DilerReport:   dilerReport,
			CRMReport:     crmReport,
			AdvisorReport: advisorReport,
			AvyuktaReport: avyuktaReport,
		})
	}

	// Step 1: Sort finalReport dynamically by branch (ascending)
	sort.Slice(finalReport, func(i, j int) bool {
		return finalReport[i].Branch < finalReport[j].Branch
	})

	// Optional: within same branch, sort by Name
	sort.SliceStable(finalReport, func(i, j int) bool {
		if finalReport[i].Branch == finalReport[j].Branch {
			return finalReport[i].Name < finalReport[j].Name
		}
		return false
	})

	return c.JSON(finalReport)
}

func DayByReportEveryStaff(c *fiber.Ctx) error {
	fromDateStr := c.Query("fromDate")
	toDateStr := c.Query("toDate")

	startOfDay, endOfDay, err := utils.ParseDateRange(fromDateStr, toDateStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid date format. Use YYYY-MM-DD"})
	}

	fmt.Println("🕐 Start:", startOfDay)
	fmt.Println("🕐 End:", endOfDay)

	staffCollection := config.GetCollection("ZoomDB", "staffs")
	callLogsCollection := config.GetCollection("ZoomDB", "calllogs")
	avyuktaCallLogs := config.GetCollection("ZoomDB", "avyuktacalls")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Create find options
	findOptions := options.Find().SetProjection(bson.M{
		"employeeId": 1,
		"name":       1,
		"branch":     1,
		"profile":    1,
	})

	// Filter: exclude role = "block"
	// filter := bson.M{
	// 	"role": bson.M{
	// 		"$ne": "block", // not equal
	// 	},
	// }

	filter := bson.M{
		"$and": []bson.M{
			{"role": bson.M{"$ne": "block"}},
			{"profile": bson.M{"$ne": "admin"}},
		},
	}

	cursor, err := staffCollection.Find(ctx, filter, findOptions)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch staff"})
	}

	var staffList []models.Staff
	if err := cursor.All(ctx, &staffList); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode staff"})
	}

	var finalReport []StaffDailyReport

	// Example Loop
	// Step 3: Loop through each employee
	for _, s := range staffList {
		empID := s.EmployeeID // <-- use struct field
		name := s.Name
		branch := s.Branch
		profile := s.Profile

		var (
			advisingNumbers []string
			crmNumbers      []string
			zoomNumbers     []string
			dialerNumbers   []string
		)

		wg := sync.WaitGroup{}
		wg.Add(4)

		go func() {
			defer wg.Done()
			advisingNumbers, _ = (&AdvisingController{}).GetAdvisingNumbersByEmployeeID(empID)
		}()
		go func() {
			defer wg.Done()
			crmNumbers, _ = (&crmLeadsController{}).GetCRMLeadsNumbersByEmployeeID(empID)
		}()
		go func() {
			defer wg.Done()
			zoomNumbers, _ = (&ClientLeadsController{}).GetClientLeadsNumbersByEmployeeID(empID)
		}()

		go func() {
			defer wg.Done()
			dialerNumbers, _ = (&DialerLeadsController{}).GetDialerLeadsNumbersByEmployeeID(empID)
		}()

		wg.Wait()

		// Combine both slices
		allNumbers := append(zoomNumbers, dialerNumbers...)
		// Optional: remove duplicates
		allNumbers = removeDuplicates(allNumbers)

		// ✅ Calculate each report section
		dilerReport, _ := getDailyCallReport(callLogsCollection, empID, allNumbers, startOfDay, endOfDay)
		crmReport, _ := getDailyCallReport(callLogsCollection, empID, crmNumbers, startOfDay, endOfDay)
		advisorReport, _ := getDailyCallReport(callLogsCollection, empID, advisingNumbers, startOfDay, endOfDay)
		avyuktaReport, _ := getDailyAvyuktaCallSummary(avyuktaCallLogs, name, startOfDay, endOfDay)
		// attendee := getAllAttendeeCount(name, startOfDay, endOfDay)
		attendee, totalAttendees := getAttendeeCounts(name, startOfDay, endOfDay)
		sales := getSalesReport(name, startOfDay, endOfDay)
		yearSale := getSalesReportByYear(name)

		finalReport = append(finalReport, StaffDailyReport{
			Name:          name,
			Branch:        branch,
			EmployeeID:    empID,
			Profile:       profile,
			Attendee:      attendee,
			TotalAttendee: totalAttendees,
			Sales:         sales,
			YearSale:      yearSale,
			DilerReport:   dilerReport,
			CRMReport:     crmReport,
			AdvisorReport: advisorReport,
			AvyuktaReport: avyuktaReport,
		})
	}

	// Step 1: Sort finalReport dynamically by branch (ascending)
	sort.Slice(finalReport, func(i, j int) bool {
		return finalReport[i].Branch < finalReport[j].Branch
	})

	// Optional: within same branch, sort by Name
	sort.SliceStable(finalReport, func(i, j int) bool {
		if finalReport[i].Branch == finalReport[j].Branch {
			return finalReport[i].Name < finalReport[j].Name
		}
		return false
	})

	return c.JSON(finalReport)
}

func getAvyuktaCallReport(callLogsCollection *mongo.Collection, EmployeeName string, start, end time.Time) ReportBlock {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{
		"full_name": EmployeeName,
		"call_date": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}

	// Create aggregation pipeline
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$sort", Value: bson.M{"call_date": 1}}},
	}

	cursor, err := callLogsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		fmt.Println("Error fetching logs:", err)
		return ReportBlock{}
	}

	defer cursor.Close(ctx)

	var logs []bson.M
	cursor.All(ctx, &logs)

	if len(logs) == 0 {
		return ReportBlock{}
	}

	totalCount := len(logs)
	totalDuration := 0
	nonZero := 0
	zero := 0

	// Loop through logs to calculate durations
	for _, log := range logs {
		durationAny, ok := log["lenth_in_sec"]
		if !ok {
			continue
		}

		var duration int
		switch v := durationAny.(type) {
		case int32:
			duration = int(v)
		case int64:
			duration = int(v)
		case float64:
			duration = int(v)
		}

		totalDuration += duration
		if duration > 0 {
			nonZero++
		} else {
			zero++
		}
	}
	// First and last call (sorted already)
	firstCall := logs[0]
	lastCall := logs[len(logs)-1]

	return ReportBlock{
		TotalCount:           totalCount,
		NonZeroDurationCount: nonZero,
		ZeroDurationCount:    zero,
		TotalDuration:        totalDuration,
		FirstCallObject:      firstCall,
		LastCallObject:       lastCall,
	}
}

func getCallReport(callLogsCollection *mongo.Collection, employeeID string, numbers []string, start, end time.Time) ReportBlock {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{
		"employeeId": employeeID,
		"timestamp": bson.M{
			"$gte": start,
			"$lte": end,
		},
		"phoneNumber": bson.M{"$in": numbers},
	}

	pipeline := mongo.Pipeline{
		{{"$match", filter}},
		{{"$sort", bson.M{"timestamp": 1}}},
	}

	cursor, err := callLogsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		fmt.Println("Error fetching logs:", err)
		return ReportBlock{}
	}
	var logs []bson.M
	cursor.All(ctx, &logs)

	if len(logs) == 0 {
		return ReportBlock{}
	}

	totalCount := len(logs)
	totalDuration := 0
	nonZero := 0
	zero := 0

	for _, log := range logs {
		durStr, _ := log["duration"].(string)
		dur, _ := strconv.Atoi(durStr)
		totalDuration += dur
		if dur > 0 {
			nonZero++
		} else {
			zero++
		}
	}

	firstCall := logs[0]
	lastCall := logs[len(logs)-1]

	return ReportBlock{
		TotalCount:           totalCount,
		NonZeroDurationCount: nonZero,
		ZeroDurationCount:    zero,
		TotalDuration:        totalDuration,
		FirstCallObject:      firstCall,
		LastCallObject:       lastCall,
	}
}

func getDailyCallReport(callLogsCollection *mongo.Collection, employeeID string, numbers []string, start, end time.Time) ([]EveryDayReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Match filter
	matchStage := bson.D{{Key: "$match", Value: bson.M{
		"employeeId":  employeeID,
		"timestamp":   bson.M{"$gte": start, "$lte": end},
		"phoneNumber": bson.M{"$in": numbers},
	}}}

	// Convert timestamp to date string (DD-MM format)
	addFieldsStage := bson.D{{Key: "$addFields", Value: bson.M{
		"dateStr": bson.M{
			"$dateToString": bson.M{
				"format": "%d-%m-%Y",
				"date":   "$timestamp",
			},
		},
	}}}

	// Group by that formatted date
	groupStage := bson.D{{
		Key: "$group",
		Value: bson.M{
			"_id": "$dateStr",
			"totalTime": bson.M{
				"$sum": bson.M{
					"$toInt": "$duration",
				},
			},
		},
	}}

	// Sort by date
	sortStage := bson.D{{Key: "$sort", Value: bson.M{"_id": 1}}}

	pipeline := mongo.Pipeline{matchStage, addFieldsStage, groupStage, sortStage}

	cursor, err := callLogsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// var results []EveryDayReport
	// if err := cursor.All(ctx, &results); err != nil {
	// 	return nil, err
	// }

	// // Rename _id → Date (if needed)
	// for i := range results {
	// 	results[i].Date = results[i].Date // already mapped via bson tag
	// }

	// return results, nil

	var aggResults []EveryDayReport
	if err := cursor.All(ctx, &aggResults); err != nil {
		return nil, err
	}

	// Convert aggregation results into a map (for quick lookup)
	resultMap := make(map[string]int)
	for _, r := range aggResults {
		resultMap[r.Date] = r.TotalTime
	}

	// Fill missing days with totalTime = 0
	var finalResults []EveryDayReport
	for d := start; !d.After(end); d = d.Add(24 * time.Hour) {
		dateStr := d.Format("02-01-2006") // same format as $dateToString
		totalTime := resultMap[dateStr]
		finalResults = append(finalResults, EveryDayReport{
			Date:      dateStr,
			TotalTime: totalTime,
		})
	}

	return finalResults, nil
}

func getDailyAvyuktaCallSummary(collection *mongo.Collection, fullName string, start, end time.Time) ([]EveryDayReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stage 1: Filter between startDate and endDate
	matchStage := bson.D{{Key: "$match", Value: bson.M{
		"full_name": fullName,
		"call_date": bson.M{"$gte": start, "$lte": end},
	}}}

	// Step 2: Convert call_date to a formatted string (like "30-10")
	addFieldsStage := bson.D{{Key: "$addFields", Value: bson.M{
		"dateStr": bson.M{
			"$dateToString": bson.M{
				"format": "%d-%m-%Y",
				"date":   "$call_date",
			},
		},
	}}}

	// Step 3: Group by date (DD-MM) and sum total time
	groupStage := bson.D{{Key: "$group", Value: bson.M{
		"_id":       "$dateStr",
		"totalTime": bson.M{"$sum": "$lenth_in_sec"},
	}}}

	// Step 4: Sort by date ascending
	sortStage := bson.D{{Key: "$sort", Value: bson.M{"_id": 1}}}

	pipeline := mongo.Pipeline{matchStage, addFieldsStage, groupStage, sortStage}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// var results []EveryDayReport
	// if err := cursor.All(ctx, &results); err != nil {
	// 	return nil, err
	// }

	// Rename _id to date for clean output
	// for i := range results {
	// 	results[i]["date"] = results[i]["_id"]
	// 	delete(results[i], "_id")
	// }

	// return results, nil

	var aggResults []EveryDayReport
	if err := cursor.All(ctx, &aggResults); err != nil {
		return nil, err
	}

	// Convert aggregation results into a map for quick lookup
	resultMap := make(map[string]int)
	for _, r := range aggResults {
		resultMap[r.Date] = r.TotalTime
	}

	// Generate all dates between start and end
	var fullResults []EveryDayReport
	for d := start; !d.After(end); d = d.Add(24 * time.Hour) {
		dateStr := d.Format("02-01-2006") // same format used in $dateToString
		totalTime := resultMap[dateStr]
		fullResults = append(fullResults, EveryDayReport{
			Date:      dateStr,
			TotalTime: totalTime,
		})
	}

	return fullResults, nil
}

// func getAllAttendeeCount(name string, start, end time.Time) int {
// 	collection := config.GetCollection("ZoomDB", "attendees") // change this to your actual collection name

// 	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
// 	defer cancel()

// 	filter := bson.M{
// 		"Team": name,
// 		"Date": bson.M{
// 			"$gte": start,
// 			"$lte": end,
// 		},
// 	}

// 	cursor, err := collection.Find(ctx, filter)
// 	if err != nil {
// 		fmt.Println("Error fetching data:", err)
// 		return 0
// 	}
// 	defer cursor.Close(ctx)

// 	var results []bson.M
// 	if err := cursor.All(ctx, &results); err != nil {
// 		fmt.Println("Error decoding data:", err)
// 		return 0
// 	}

// 	total := 0
// 	for _, doc := range results {
// 		if val, ok := doc["Attendees"].(int32); ok {
// 			total += int(val)
// 		} else if val, ok := doc["Attendees"].(int64); ok {
// 			total += int(val)
// 		} else if val, ok := doc["Attendees"].(float64); ok {
// 			total += int(val)
// 		}
// 	}

// 	return total
// }

func getAttendeeCounts(team string, start, end time.Time) (int, int) {
	collection := config.GetCollection("ZoomDB", "attendees")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// --- Pipeline 1: Sum between start and end ---
	pipelineRange := mongo.Pipeline{
		{{"$match", bson.D{
			{"Team", team},
			{"Date", bson.D{
				{"$gte", start},
				{"$lte", end},
			}},
		}}},
		{{"$group", bson.D{
			{"_id", nil},
			{"total", bson.D{{"$sum", "$Attendees"}}},
		}}},
	}

	var rangeResult []bson.M
	rangeCur, _ := collection.Aggregate(ctx, pipelineRange)
	rangeCur.All(ctx, &rangeResult)

	var dateRangeAttendees int
	// if len(rangeResult) > 0 {
	// 	dateRangeAttendees = rangeResult[0]["total"].(int)
	// }

	if len(rangeResult) > 0 {
		switch v := rangeResult[0]["total"].(type) {
		case int32:
			dateRangeAttendees = int(v)
		case int64:
			dateRangeAttendees = int(v)
		case float64:
			dateRangeAttendees = int(v)
		}
	}

	// --- Pipeline 2: Sum all attendees for that team (no date filter) ---
	pipelineTotal := mongo.Pipeline{
		{{"$match", bson.D{
			{"Team", team},
		}}},
		{{"$group", bson.D{
			{"_id", nil},
			{"total", bson.D{{"$sum", "$Attendees"}}},
		}}},
	}

	var totalResult []bson.M
	totalCur, _ := collection.Aggregate(ctx, pipelineTotal)
	totalCur.All(ctx, &totalResult)

	var totalAttendees int
	// if len(totalResult) > 0 {
	// 	totalAttendees = totalResult[0]["total"].(int)
	// }

	if len(totalResult) > 0 {
		switch v := totalResult[0]["total"].(type) {
		case int32:
			totalAttendees = int(v)
		case int64:
			totalAttendees = int(v)
		case float64:
			totalAttendees = int(v)
		}
	}

	return dateRangeAttendees, totalAttendees
}

func getSalesReport(name string, start, end time.Time) map[string]int {

	collection := config.GetCollection("ZoomDB", "salesleads")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Date filter
	filter := bson.M{
		"Date of Enrollment": bson.M{
			"$gte": start,
			"$lte": end,
		},
		// Only documents where L1 or L2/L3 CONTAINS the name
		"$or": []bson.M{
			{"L1": bson.M{"$regex": name, "$options": "i"}},
			{"L2/L3": bson.M{"$regex": name, "$options": "i"}},
		},
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		fmt.Println("Error fetching:", err)
		return nil
	}
	defer cursor.Close(ctx)

	var results []SalesLead
	if err := cursor.All(ctx, &results); err != nil {
		fmt.Println("Decode error:", err)
		return nil
	}

	count := map[string]int{
		"L1":   0,
		"L2L3": 0,
	}

	for _, r := range results {

		// If L1 contains given name
		if strings.Contains(strings.ToLower(r.L1), strings.ToLower(name)) {
			count["L1"]++
		}

		// If L2/L3 contains given name
		if strings.Contains(strings.ToLower(r.L2L3), strings.ToLower(name)) {
			count["L2L3"]++
		}
	}

	return count
}

func getSalesReportByYear(name string) map[string]map[string]int {

	collection := config.GetCollection("ZoomDB", "salesleads")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pipeline := mongo.Pipeline{
		{{"$match", bson.D{
			{"$or", bson.A{
				bson.D{{"L1", bson.D{{"$regex", name}, {"$options", "i"}}}},
				bson.D{{"L2/L3", bson.D{{"$regex", name}, {"$options", "i"}}}},
			}},
		}}},
		{{"$group", bson.D{
			{"_id", "$Year"},
			{"L1", bson.D{{"$sum", bson.D{
				{"$cond", bson.A{
					bson.D{{"$regexMatch", bson.D{{"input", "$L1"}, {"regex", name}, {"options", "i"}}}},
					1, 0,
				}},
			}}}},
			{"L2L3", bson.D{{"$sum", bson.D{
				{"$cond", bson.A{
					bson.D{{"$regexMatch", bson.D{{"input", "$L2/L3"}, {"regex", name}, {"options", "i"}}}},
					1, 0,
				}},
			}}}},
		}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		fmt.Println("Aggregation error:", err)
		return nil
	}

	var results []YearSales
	if err := cursor.All(ctx, &results); err != nil {
		fmt.Println("Cursor decode error:", err)
		return nil
	}

	final := map[string]map[string]int{}

	for _, r := range results {
		final[r.ID] = map[string]int{
			"L1":   r.L1,
			"L2L3": r.L2L3,
		}
	}

	return final
}

func cleanName(value string) string {
	re := regexp.MustCompile(`\s+(L\d(\/\d)?|OV).*`)
	return strings.TrimSpace(re.ReplaceAllString(value, ""))
}

// GetAdvisingNumbersByEmployeeID returns all phone numbers for a given employeeid
func (c *AdvisingController) GetAdvisingNumbersByEmployeeID(employeeID string) ([]string, error) {
	advisingCollection := config.GetCollection("ZoomDB", "advisingleads")

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// fmt.Println(employeeID)

	// Find all documents with this employeeid
	cursor, err := advisingCollection.Find(timeoutCtx, bson.M{"employeeid": employeeID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(timeoutCtx)

	var results []bson.M
	if err := cursor.All(timeoutCtx, &results); err != nil {
		return nil, err
	}

	// Loop through and collect phone numbers
	var advisingNumbers []string
	for _, doc := range results {
		for _, field := range []string{"phone1", "phone2", "phone3", "phone4"} {
			if val, ok := doc[field].(string); ok && val != "" {
				advisingNumbers = append(advisingNumbers, val)
			}
		}
	}

	// Remove duplicates
	uniqueNumbers := removeDuplicates(advisingNumbers)

	return uniqueNumbers, nil
}

// GetCRMLeadsByEmployeeID returns all phone numbers for a given employeeid
func (c *crmLeadsController) GetCRMLeadsNumbersByEmployeeID(employeeID string) ([]string, error) {
	crmCollection := config.GetCollection("ZoomDB", "crmleads")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// fmt.Println(employeeID)

	empIDInt64, err := strconv.ParseInt(employeeID, 10, 64)
	if err != nil {
		return nil, err
	}

	// Find all documents for this employee
	cursor, err := crmCollection.Find(ctx, bson.M{"employeeid": empIDInt64})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// fmt.Println(cursor)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// fmt.Println(results)

	// Loop through results and get mobile numbers
	var mobileNumbers []string
	for _, doc := range results {
		if val, ok := doc["mobile"]; ok {
			switch v := val.(type) {
			case int:
				mobileNumbers = append(mobileNumbers, fmt.Sprintf("%d", v))
			case int32:
				mobileNumbers = append(mobileNumbers, fmt.Sprintf("%d", v))
			case int64:
				mobileNumbers = append(mobileNumbers, fmt.Sprintf("%d", v))
			case float64:
				mobileNumbers = append(mobileNumbers, fmt.Sprintf("%.0f", v))
			case string:
				mobileNumbers = append(mobileNumbers, v)
			}
		}
	}

	// Optional: remove duplicates
	mobileNumbers = removeDuplicates(mobileNumbers)

	return mobileNumbers, nil
}

func (c *ClientLeadsController) GetClientLeadsNumbersByEmployeeID(employeeID string) ([]string, error) {
	clientCollection := config.GetCollection("ZoomDB", "clients")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Find all documents for this employeeId
	// cursor, err := clientCollection.Find(ctx, bson.M{"employeeId": employeeID})
	cursor, err := clientCollection.Find(ctx,
		bson.M{"employeeId": employeeID},
		options.Find().SetProjection(bson.M{"follow_up": 1, "parentNumber": 1, "number": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	var allNumbers []string
	for _, doc := range results {
		for _, field := range []string{"follow_up", "parentNumber", "number"} {
			if val, ok := doc[field]; ok {
				switch v := val.(type) {
				case string:
					if v != "" {
						allNumbers = append(allNumbers, v)
					}
				case int, int32, int64:
					allNumbers = append(allNumbers, fmt.Sprintf("%d", v))
				case float64:
					allNumbers = append(allNumbers, fmt.Sprintf("%.0f", v))
				}
			}
		}
	}

	allNumbers = removeDuplicates(allNumbers)
	return allNumbers, nil
}

func (c *DialerLeadsController) GetDialerLeadsNumbersByEmployeeID(employeeID string) ([]string, error) {
	dialerCollection := config.GetCollection("ZoomDB", "dialerleads")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Find all documents for this employee
	cursor, err := dialerCollection.Find(ctx, bson.M{"employeeid": employeeID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// fmt.Println(cursor)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// Extract all mobile numbers
	var mobileNumbers []string
	for _, doc := range results {
		if val, ok := doc["mobile"]; ok {
			mobileNumbers = append(mobileNumbers, fmt.Sprintf("%v", val))
		}
	}

	// Optional: remove duplicates
	mobileNumbers = removeDuplicates(mobileNumbers)

	return mobileNumbers, nil
}

func removeDuplicates(arr []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range arr {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
