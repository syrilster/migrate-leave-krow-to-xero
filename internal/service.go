package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ses"
	"gopkg.in/gomail.v2"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
	log "github.com/sirupsen/logrus"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/model"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/xero"
)

const (
	unPaidLeave        string = "Other Unpaid Leave"
	compassionateLeave string = "Compassionate Leave (paid)"
	juryDutyLeave      string = "Jury Duty"
	personalLeave      string = "Personal/Carer's Leave"
	annualLeave        string = "Annual Leave"

	annualLeaveNegativeLimit   float64 = -40
	personalLeaveNegativeLimit float64 = -16
)

type Service struct {
	client          xero.ClientInterface
	xlsFileLocation string
	emailClient     *ses.SES
	emailTo         string
}

type EmpLeaveRequest struct {
	empID             string
	empName           string
	tenantID          string
	leaveTypeID       string
	leaveUnits        float64
	paymentDate       string
	leaveStartDate    string
	leaveEndDate      string
	leaveType         string
	leaveDate         string
	originalLeaveType string
	orgName           string
}

func NewService(c xero.ClientInterface, xlsLocation string, ec *ses.SES, emailTo string) *Service {
	return &Service{
		client:          c,
		xlsFileLocation: xlsLocation,
		emailClient:     ec,
		emailTo:         emailTo,
	}
}

//MigrateLeaveKrowToXero func will process the leave requests
func (service Service) MigrateLeaveKrowToXero(ctx context.Context) []string {
	var errResult []string
	var successResult []string
	var errStrings []error
	var wg sync.WaitGroup
	var xeroEmployeesMap map[string]xero.Employee
	var payrollCalendarMap = make(map[string]string)
	var connectionsMap = make(map[string]string)
	var resultChan = make(chan string)

	ctxLogger := log.WithContext(ctx)
	ctxLogger.Infof("Executing MigrateLeaveKrowToXero service")

	leaveRequests, err := service.extractDataFromKrow(ctx)
	if err != nil {
		errResult = append(errResult, err.Error())
		service.sendStatusReport(ctx, errResult, successResult)
		return errResult
	}

	resp, err := service.client.GetConnections(ctx)
	if err != nil {
		errStr := fmt.Errorf("Failed to fetch connections from Xero: %v ", err)
		ctxLogger.Infof(errStr.Error())
		errResult = append(errResult, errStr.Error())
		service.sendStatusReport(ctx, errResult, successResult)
		return errResult
	}

	for _, c := range resp {
		connectionsMap[c.OrgName] = c.TenantID
	}

	for _, leaveReq := range leaveRequests {
		orgName := leaveReq.OrgName
		empName := leaveReq.EmpName
		xeroEmployeesMap = make(map[string]xero.Employee)
		if _, ok := connectionsMap[orgName]; !ok {
			errStr := fmt.Errorf("Failed to get ORG details from Xero. OrgName: %v ", orgName)
			ctxLogger.Infof(errStr.Error())
			errStrings = append(errStrings, errStr)
			continue
		}

		tenantID := connectionsMap[orgName]
		empResponse, err := service.client.GetEmployees(ctx, tenantID)
		if err != nil {
			errStr := fmt.Errorf("Failed to fetch employees from Xero. OrgName: %v ", orgName)
			ctxLogger.Infof(err.Error(), err)
			errResult = append(errResult, errStr.Error())
			continue
		}

		//populate the employees to a map
		for _, emp := range empResponse.Employees {
			xeroEmployeesMap[emp.FirstName+" "+emp.LastName] = emp
		}

		payCalendarResp, err := service.client.GetPayrollCalendars(ctx, tenantID)
		if err != nil {
			errStr := fmt.Errorf("Failed to fetch employee payroll calendar settings from Xero. OrgName: %v ", orgName)
			ctxLogger.Infof(err.Error(), err)
			errStrings = append(errStrings, errStr)
			continue
		}

		//Populate the payroll settings to a map
		for _, p := range payCalendarResp.PayrollCalendars {
			payrollCalendarMap[p.PayrollCalendarID] = p.PaymentDate
		}

		errStr := service.processLeaveRequestByEmp(ctx, xeroEmployeesMap, empName, orgName, leaveReq, tenantID, payrollCalendarMap, resultChan, &wg)
		if errStr != nil {
			if !containsError(errStrings, errStr.Error()) {
				errStrings = append(errStrings, errStr)
			}
		}
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for _, e := range errStrings {
		if e.Error() != "" {
			errResult = append(errResult, e.Error())
		}
	}
	for result := range resultChan {
		if strings.Contains(result, "Error:") {
			errResult = append(errResult, result)
		} else {
			successResult = append(successResult, result)
		}
	}

	service.sendStatusReport(ctx, errResult, successResult)
	if len(errResult) > 0 {
		return errResult
	}
	return nil
}

func (service Service) sendStatusReport(ctx context.Context, errResult []string, result []string) {
	resultString := strings.Join(result, "\n")
	errorsString := strings.Join(errResult, "\n")
	if errorsString == "" {
		errorsString = "No errors found during processing leaves. Please check attached report for audit trail."
	}
	go service.sesSendEmail(ctx, resultString, errorsString)
}

func (service Service) processLeaveRequestByEmp(ctx context.Context, xeroEmployeesMap map[string]xero.Employee,
	empName string, orgName string, leaveReq model.KrowLeaveRequest, tenantID string, payrollCalendarMap map[string]string,
	resChan chan string, wg *sync.WaitGroup) error {
	ctxLogger := log.WithContext(ctx)

	if _, ok := xeroEmployeesMap[empName]; !ok {
		errStr := fmt.Errorf("Employee not found in Xero. Employee Name: %v ", empName)
		ctxLogger.Infof(errStr.Error())
		return errStr
	}
	empID := xeroEmployeesMap[empName].EmployeeID
	payCalendarID := xeroEmployeesMap[empName].PayrollCalendarID
	if _, ok := payrollCalendarMap[payCalendarID]; !ok {
		errStr := fmt.Errorf("Failed to fetch employee payroll calendar settings from Xero. Employee Name: %v ", empName)
		ctxLogger.Infof(errStr.Error())
		return errStr
	}
	paymentDate := payrollCalendarMap[payCalendarID]
	leaveBalance, err := service.client.EmployeeLeaveBalance(ctx, tenantID, empID)
	if err != nil {
		errStr := fmt.Errorf("Failed to fetch employee leave balance from Xero. Emp Name: %v ", empName)
		ctxLogger.Infof(errStr.Error(), err)
		return errStr
	}
	err = service.reconcileLeaveRequestAndApply(ctx, empID, empName, orgName, tenantID, leaveReq, paymentDate, leaveBalance, resChan, wg)
	return err
}

func (service Service) reconcileLeaveRequestAndApply(ctx context.Context, empID string, empName string, orgName string,
	tenantID string, leaveReq model.KrowLeaveRequest, paymentDate string, leaveBalance *xero.LeaveBalanceResponse,
	resChan chan string, wg *sync.WaitGroup) error {
	var leaveBalanceMap = make(map[string]xero.LeaveBalance)
	var leaveTypeID string
	var leaveStartDate string
	var leaveEndDate string
	var unpaidLeaveUnits float64
	var paidLeaveUnits float64
	var unPaidLeaveTypeID string
	var errorsStr []string
	var skipUnpaidLeave bool
	var negativeLeaveLimit float64

	ctxLogger := log.WithContext(ctx)
	ctxLogger.Infof("Calculating leaves to be applied for Employee %v", empName)

	skipUnpaidLeave = strings.EqualFold(leaveReq.LeaveType, compassionateLeave) || strings.EqualFold(leaveReq.LeaveType, juryDutyLeave)
	for _, leaveBal := range leaveBalance.Employees[0].LeaveBalance {
		leaveBalanceMap[leaveBal.LeaveType] = leaveBal
		if strings.EqualFold(leaveBal.LeaveType, unPaidLeave) {
			unPaidLeaveTypeID = leaveBal.LeaveTypeID
		}
	}

	if _, ok := leaveBalanceMap[leaveReq.LeaveType]; !ok {
		errStr := fmt.Errorf("Leave type %v not found/configured in Xero for Employee Name: %v ", leaveReq.LeaveType, empName)
		ctxLogger.Infof(errStr.Error())
		errorsStr = append(errorsStr, errStr.Error())
		return errStr
	}

	lb := leaveBalanceMap[leaveReq.LeaveType]
	leaveReqUnit := leaveReq.Hours
	availableLeaveBalUnit := lb.NumberOfUnits
	leaveTypeID = lb.LeaveTypeID
	leaveStartDate = "/Date(" + strconv.FormatInt(leaveReq.LeaveDateEpoch, 10) + ")/"
	leaveEndDate = "/Date(" + strconv.FormatInt(leaveReq.LeaveDateEpoch, 10) + ")/"
	//Special case for annual leave and personal leave i.e negative leave allowed
	if strings.EqualFold(leaveReq.LeaveType, annualLeave) || strings.EqualFold(leaveReq.LeaveType, personalLeave) {
		if strings.EqualFold(leaveReq.LeaveType, personalLeave) {
			negativeLeaveLimit = personalLeaveNegativeLimit
		} else {
			negativeLeaveLimit = annualLeaveNegativeLimit
		}
		//To handle a edge case if leave is for ex: -44 then reset to zero
		if availableLeaveBalUnit < negativeLeaveLimit {
			availableLeaveBalUnit = 0
		} else if availableLeaveBalUnit > 0 {
			//To handle a case when leave is positive for ex:20
			availableLeaveBalUnit = math.Abs(negativeLeaveLimit) + availableLeaveBalUnit
		} else {
			//leave already in negative
			availableLeaveBalUnit = math.Abs(negativeLeaveLimit - availableLeaveBalUnit)
		}
	}
	if leaveReqUnit >= availableLeaveBalUnit {
		if availableLeaveBalUnit > 0 {
			paidLeaveUnits = availableLeaveBalUnit
			unpaidLeaveUnits += leaveReqUnit - availableLeaveBalUnit
		} else {
			//Employees has negative or zero leave balance and hence unpaid leave
			paidLeaveUnits = 0
			unpaidLeaveUnits += leaveReqUnit
		}
	} else {
		paidLeaveUnits = leaveReqUnit
	}

	if paidLeaveUnits > 0 {
		wg.Add(1)
		paidLeaveReq := EmpLeaveRequest{
			empID:             empID,
			empName:           empName,
			tenantID:          tenantID,
			leaveTypeID:       leaveTypeID,
			leaveUnits:        paidLeaveUnits,
			paymentDate:       paymentDate,
			leaveStartDate:    leaveStartDate,
			leaveEndDate:      leaveEndDate,
			leaveType:         leaveReq.LeaveType,
			leaveDate:         leaveReq.LeaveDate.Format("2/1/2006"),
			originalLeaveType: leaveReq.LeaveType,
			orgName:           orgName,
		}
		service.applyLeave(ctx, paidLeaveReq, resChan, wg)
	}

	if unpaidLeaveUnits > 0 && !skipUnpaidLeave {
		wg.Add(1)
		unPaidLeaveReq := EmpLeaveRequest{
			empID:             empID,
			empName:           empName,
			tenantID:          tenantID,
			leaveTypeID:       unPaidLeaveTypeID,
			leaveUnits:        unpaidLeaveUnits,
			paymentDate:       paymentDate,
			leaveStartDate:    leaveStartDate,
			leaveEndDate:      leaveEndDate,
			leaveType:         unPaidLeave,
			leaveDate:         leaveReq.LeaveDate.Format("2/1/2006"),
			originalLeaveType: leaveReq.LeaveType,
			orgName:           orgName,
		}
		service.applyLeave(ctx, unPaidLeaveReq, resChan, wg)
	}

	if unpaidLeaveUnits > 0 && skipUnpaidLeave {
		errStr := fmt.Errorf("Employee: %v has insufficient Leave balance for Leave type %v requested for %v hours ", empName, leaveReq.LeaveType, unpaidLeaveUnits)
		errorsStr = append(errorsStr, errStr.Error())
	}

	e := strings.Join(errorsStr, "\n")
	errRes := errors.New(e)
	return errRes
}

func (service Service) applyLeave(ctx context.Context, leaveReq EmpLeaveRequest, resChan chan string, wg *sync.WaitGroup) {
	var leavePeriods = make([]xero.LeavePeriod, 1)
	leavePeriod := xero.LeavePeriod{
		PayPeriodEndDate: leaveReq.paymentDate,
		NumberOfUnits:    leaveReq.leaveUnits,
	}

	leavePeriods[0] = leavePeriod
	leaveApplication := xero.LeaveApplicationRequest{
		EmployeeID:   leaveReq.empID,
		LeaveTypeID:  leaveReq.leaveTypeID,
		StartDate:    leaveReq.leaveStartDate,
		EndDate:      leaveReq.leaveEndDate,
		Title:        leaveReq.leaveType,
		LeavePeriods: leavePeriods,
	}
	go service.applyLeaveRequestToXero(ctx, leaveReq.tenantID, leaveReq.leaveType, leaveReq.originalLeaveType,
		leaveReq.leaveDate, leaveApplication, leaveReq.empName, leaveReq.orgName, resChan, wg)
}

func (service Service) applyLeaveRequestToXero(ctx context.Context, tenantID string, appliedLeaveType string, originalLeaveType string,
	leaveDate string, leaveApplication xero.LeaveApplicationRequest, empName string, orgName string, resChan chan string, wg *sync.WaitGroup) {
	ctxLogger := log.WithContext(ctx)
	ctxLogger.Infof("Applying leave request for Employees: %v", empName)

	defer func() {
		wg.Done()
	}()

	err := service.client.EmployeeLeaveApplication(ctx, tenantID, leaveApplication)
	if err != nil {
		ctxLogger.Infof("Leave Application Request: %v", leaveApplication)
		ctxLogger.WithError(err).Errorf("Failed to post Leave application to xero for employee: %v ", empName)
		resChan <- fmt.Sprintf("Error: Failed to post Leave application to xero for employee %v. Check if entered leave type/unpaid leave is configured. ", empName)
		return
	}
	resChan <- fmt.Sprintf("%v,%v,%v,%v,%v,%v",
		empName, originalLeaveType, appliedLeaveType, leaveDate, leaveApplication.LeavePeriods[0].NumberOfUnits, orgName)
}

func (service Service) extractDataFromKrow(ctx context.Context) ([]model.KrowLeaveRequest, error) {
	var leaveRequests []model.KrowLeaveRequest
	ctxLogger := log.WithContext(ctx)

	f, err := excelize.OpenFile(service.xlsFileLocation)
	if err != nil {
		errStr := fmt.Errorf("Unable to open the uploaded file. Please confirm the file is in xlsx format. ")
		ctxLogger.WithError(err).Error(errStr)
		return nil, errStr
	}

	rows, err := f.GetRows("Sheet1")
	for index, row := range rows {
		if index == 0 {
			continue
		}
		leaveDate, err := time.Parse("2/1/2006", row[1])
		if err != nil {
			errStr := fmt.Errorf("Error while parsing leave date for entry: %v ", row[1])
			ctxLogger.WithError(err).Error(errStr)
			return nil, errStr
		}
		hours, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			errStr := fmt.Errorf("Error while parsing leave hours for entry: %v ", row[2])
			ctxLogger.WithError(err).Error(errStr)
			return nil, errStr
		}
		leaveType := row[5]
		if leaveType == "" {
			leaveType = row[3]
		}
		r := strings.NewReplacer("Carers", "Carer's",
			"Unpaid", "Other Unpaid",
			"Parental Leave (10 days for new family member)", "Parental Leave (Paid)",
			"Parental Leave", "Parental Leave (Paid)",
			"Compassionate Leave", "Compassionate Leave (paid)")
		leaveType = r.Replace(leaveType)
		empName := row[0]
		orgName := row[6]
		leaveReq := model.KrowLeaveRequest{
			LeaveDate:      leaveDate,
			LeaveDateEpoch: leaveDate.UnixNano() / 1000000,
			Hours:          hours,
			LeaveType:      leaveType,
			OrgName:        orgName,
			EmpName:        empName,
		}
		leaveRequests = append(leaveRequests, leaveReq)
	}
	return leaveRequests, nil
}

func (service Service) sesSendEmail(ctx context.Context, attachmentData string, data string) {
	contextLogger := log.WithContext(ctx)
	contextLogger.Infof("Inside sesSendEmail func..")
	attachFileName := "/tmp/report.xlsx"

	writeAttachmentDataToExcel(ctx, attachFileName, attachmentData)

	msg := gomail.NewMessage()
	msg.SetHeader("From", service.emailTo)
	msg.SetHeader("To", service.emailTo)
	msg.SetHeader("Subject", "Report: Leave Migration to Xero")
	msg.SetBody("text/plain", data)
	msg.Attach(attachFileName)

	var emailRaw bytes.Buffer
	_, err := msg.WriteTo(&emailRaw)
	if err != nil {
		contextLogger.WithError(err).Error("Error when writing email data")
		return
	}

	message := ses.RawMessage{Data: emailRaw.Bytes()}
	emailParams := ses.SendRawEmailInput{
		Source:       aws.String(service.emailTo),
		Destinations: []*string{aws.String(service.emailTo)},
		RawMessage:   &message,
	}

	_, err = service.emailClient.SendRawEmail(&emailParams)
	if err != nil {
		contextLogger.WithError(err).Error("Error when sending email")
		return
	}
	return
}

func writeAttachmentDataToExcel(ctx context.Context, attachFileName string, attachmentData string) {
	contextLogger := log.WithContext(ctx)
	f := excelize.NewFile()
	// Create a new sheet.
	index := f.NewSheet("Sheet1")
	_ = f.SetColWidth("Sheet1", "A", "E", 20)
	_ = f.SetColWidth("Sheet1", "B", "C", 30)
	// Set value of a cell.
	err := f.SetCellValue("Sheet1", "A1", "Employee")
	if err != nil {
		contextLogger.WithError(err)
		return
	}
	err = f.SetCellValue("Sheet1", "B1", "Leave Requested")
	if err != nil {
		contextLogger.WithError(err)
		return
	}
	err = f.SetCellValue("Sheet1", "C1", "Leave Applied (Xero)")
	if err != nil {
		contextLogger.WithError(err)
		return
	}
	err = f.SetCellValue("Sheet1", "D1", "Leave Date")
	if err != nil {
		contextLogger.WithError(err)
		return
	}
	err = f.SetCellValue("Sheet1", "E1", "Hours")
	if err != nil {
		contextLogger.WithError(err)
		return
	}
	err = f.SetCellValue("Sheet1", "F1", "Org")
	if err != nil {
		contextLogger.WithError(err)
		return
	}
	rows := strings.Split(attachmentData, "\n")
	rowStartIndex := 2
	for _, row := range rows {
		cells := strings.Split(row, ",")
		rowStartIndexStr := strconv.Itoa(rowStartIndex)
		err := f.SetCellValue("Sheet1", "A"+rowStartIndexStr, cells[0])
		if err != nil {
			contextLogger.WithError(err)
			return
		}
		err = f.SetCellValue("Sheet1", "B"+rowStartIndexStr, cells[1])
		if err != nil {
			contextLogger.WithError(err)
			return
		}
		err = f.SetCellValue("Sheet1", "C"+rowStartIndexStr, cells[2])
		if err != nil {
			contextLogger.WithError(err)
			return
		}
		err = f.SetCellValue("Sheet1", "D"+rowStartIndexStr, cells[3])
		if err != nil {
			contextLogger.WithError(err)
			return
		}
		err = f.SetCellValue("Sheet1", "E"+rowStartIndexStr, cells[4])
		if err != nil {
			contextLogger.WithError(err)
			return
		}
		err = f.SetCellValue("Sheet1", "F"+rowStartIndexStr, cells[5])
		if err != nil {
			contextLogger.WithError(err)
			return
		}
		rowStartIndex++
	}

	// Set active sheet of the workbook.
	f.SetActiveSheet(index)
	// Save xlsx file by the given path.
	if err := f.SaveAs(attachFileName); err != nil {
		fmt.Println(err)
	}
}

func containsError(errors []error, errStr string) bool {
	for _, s := range errors {
		if strings.Contains(s.Error(), errStr) {
			return true
		}
	}
	return false
}
