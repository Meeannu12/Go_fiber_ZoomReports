package models

// type Staff struct {
// 	ID         string `json:"id" bson:"_id,omitempty"`
// 	Name       string `json:"name" bson:"name"`
// 	EmployeeID string `bson:"employeeId" json:"employeeId"`
// 	Branch     string `bson:"branch" json:"branch"`
// 	Profile    string `bson:"profile" json:"profile"`
// 	// Email string `json:"email" bson:"email"`
// 	// Age   int    `json:"age" bson:"age"`
// }

type Staff struct {
	ID         string `bson:"_id,omitempty" json:"_id"`
	EmployeeID string `bson:"employeeId" json:"employeeId"`
	Name       string `bson:"name" json:"name"`
	Branch     string `bson:"branch" json:"branch"`
	Profile    string `bson:"profile", json:"profile"`
}
