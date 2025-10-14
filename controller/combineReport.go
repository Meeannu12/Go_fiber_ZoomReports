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
	Profile       string      `json:"profile`
	DilerReport   ReportBlock `json:"dilerReport"`
	CRMReport     ReportBlock `json:"crmReport"`
	AdvisorReport ReportBlock `json:"advisorReport"`
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
		)

		wg := sync.WaitGroup{}
		wg.Add(3)

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

		wg.Wait()

		// ✅ Calculate each report section
		dilerReport := getCallReport(callLogsCollection, empID, zoomNumbers, startOfDay, endOfDay)
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
