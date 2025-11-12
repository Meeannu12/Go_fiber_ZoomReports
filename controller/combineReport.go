package controller

import (
	"context"
	"fmt"
	"go_fiber_Zoom_Report/config"
	"go_fiber_Zoom_Report/models"
	"go_fiber_Zoom_Report/utils"
	"strconv"
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
	Name          string      `json:"name"`
	Branch        string      `json:"branch"`
	EmployeeID    string      `json:"employeeId"`
	Profile       string      `json:"profile"`
	DilerReport   ReportBlock `json:"dilerReport"`
	CRMReport     ReportBlock `json:"crmReport"`
	AdvisorReport ReportBlock `json:"advisorReport"`
}

type StaffDailyReport struct {
	Name          string           `json:"name"`
	Branch        string           `json:"branch"`
	EmployeeID    string           `json:"employeeId"`
	Profile       string           `json:"profile"`
	DilerReport   []EveryDayReport `json:"dilerReport"`
	CRMReport     []EveryDayReport `json:"crmReport"`
	AdvisorReport []EveryDayReport `json:"advisorReport"`
	AvyuktaReport []EveryDayReport `json:"avyuktaReport"`
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

type AdvisingController struct{}
type crmLeadsController struct{}
type ClientLeadsController struct{}
type DialerLeadsController struct{}

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Create find options
	findOptions := options.Find().SetProjection(bson.M{
		"employeeId": 1,
		"name":       1,
		"branch":     1,
		"profile":    1,
	})

	cursor, err := staffCollection.Find(ctx, bson.M{}, findOptions)

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

		// ✅ Fetch all number lists (already implemented functions)
		// advisingController := &AdvisingController{}
		// advisingNumbers, _ := advisingController.GetAdvisingNumbersByEmployeeID(empID)

		// crmController := &crmLeadsController{}
		// crmNumbers, _ := crmController.GetCRMLeadsNumbersByEmployeeID(empID)

		// zoomController := &ClientLeadsController{}
		// zoomNumbers, _ := zoomController.GetClientLeadsNumbersByEmployeeID(empID)

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
		dilerReport := getCallReport(callLogsCollection, empID, allNumbers, startOfDay, endOfDay)
		crmReport := getCallReport(callLogsCollection, empID, crmNumbers, startOfDay, endOfDay)
		advisorReport := getCallReport(callLogsCollection, empID, advisingNumbers, startOfDay, endOfDay)

		finalReport = append(finalReport, StaffReport{
			Name:          name,
			Branch:        branch,
			EmployeeID:    empID,
			Profile:       profile,
			DilerReport:   dilerReport,
			CRMReport:     crmReport,
			AdvisorReport: advisorReport,
		})
	}

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

	cursor, err := staffCollection.Find(ctx, bson.M{}, findOptions)

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

		finalReport = append(finalReport, StaffDailyReport{
			Name:          name,
			Branch:        branch,
			EmployeeID:    empID,
			Profile:       profile,
			DilerReport:   dilerReport,
			CRMReport:     crmReport,
			AdvisorReport: advisorReport,
			AvyuktaReport: avyuktaReport,
		})
	}

	return c.JSON(finalReport)
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

	// cursor, err := callLogsCollection.Find(ctx, filter)
	// if err != nil {
	// 	fmt.Println("Error fetching logs:", err)
	// 	return ReportBlock{}
	// }
	// defer cursor.Close(ctx)

	// var logs []bson.M
	// cursor.All(ctx, &logs)

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

	var results []EveryDayReport
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// Rename _id → Date (if needed)
	for i := range results {
		results[i].Date = results[i].Date // already mapped via bson tag
	}

	return results, nil
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

	var results []EveryDayReport
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// Rename _id to date for clean output
	// for i := range results {
	// 	results[i]["date"] = results[i]["_id"]
	// 	delete(results[i], "_id")
	// }

	return results, nil
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

	// fmt.Println(results)

	// Loop through results and get mobile numbers
	// var mobileNumbers []string
	// for _, doc := range results {
	// 	if val, ok := doc["mobile"]; ok {
	// 		switch v := val.(type) {
	// 		case int:
	// 			mobileNumbers = append(mobileNumbers, fmt.Sprintf("%d", v))
	// 		case int32:
	// 			mobileNumbers = append(mobileNumbers, fmt.Sprintf("%d", v))
	// 		case int64:
	// 			mobileNumbers = append(mobileNumbers, fmt.Sprintf("%d", v))
	// 		case float64:
	// 			mobileNumbers = append(mobileNumbers, fmt.Sprintf("%.0f", v))
	// 		case string:
	// 			mobileNumbers = append(mobileNumbers, v)
	// 		}
	// 	}
	// }

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
