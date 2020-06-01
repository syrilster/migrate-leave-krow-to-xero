package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
	log "github.com/sirupsen/logrus"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/model"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/xero"
)

const digIO = "DigIO"
const cmd = "CMD"
const eliiza = "Eliiza"
const kasna = "Kasna"
const mantel = "Mantel Group"
const unPaidLeave = "Other Unpaid Leave"

type Service struct {
	client          xero.ClientInterface
	xlsFileLocation string
}

type EmpLeaveRequest struct {
	empID          string
	empName        string
	tenantID       string
	leaveTypeID    string
	leaveUnits     float64
	paymentDate    string
	leaveStartDate string
	leaveEndDate   string
	leaveType      string
}

func NewService(c xero.ClientInterface, xlsLocation string) *Service {
	return &Service{
		client:          c,
		xlsFileLocation: xlsLocation,
	}
}

//MigrateLeaveKrowToXero func will process the leave requests
func (service Service) MigrateLeaveKrowToXero(ctx context.Context) []string {
	var errResult []string
	var errStrings []error
	var wg sync.WaitGroup
	var xeroEmployeesMap map[string]xero.Employee
	var payrollCalendarMap = make(map[string]string)
	var connectionsMap = make(map[string]string)
	var errChan = make(chan error)

	ctxLogger := log.WithContext(ctx)
	ctxLogger.Infof("Executing MigrateLeaveKrowToXero service")

	leaveRequests, err := service.extractDataFromKrow(ctx)
	if err != nil {
		errResult = append(errResult, err.Error())
		return errResult
	}

	resp, err := service.client.GetConnections(ctx)
	if err != nil {
		errStr := fmt.Errorf("failed to fetch connections from Xero: %v", err)
		ctxLogger.Infof(errStr.Error())
		errResult = append(errResult, errStr.Error())
		return errResult
	}
	for _, c := range resp {
		connectionsMap[c.OrgName] = c.TenantID
	}

	for orgName, leaveReqMap := range leaveRequests {
		if len(leaveReqMap) == 0 {
			continue
		}
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
			errStr := fmt.Errorf("failed to fetch employees from Xero. OrgName: %v ", orgName)
			ctxLogger.Infof(err.Error(), err)
			errResult = append(errResult, errStr.Error())
			return errResult
		}

		//populate the employees to a map
		for _, emp := range empResponse.Employees {
			xeroEmployeesMap[emp.FirstName+" "+emp.LastName] = emp
		}

		payCalendarResp, err := service.client.GetPayrollCalendars(ctx, tenantID)
		if err != nil {
			errStr := fmt.Errorf("failed to fetch employee payroll calendar settings from Xero.OrgName: %v ", orgName)
			ctxLogger.Infof(err.Error(), err)
			errStrings = append(errStrings, errStr)
			continue
		}

		//Populate the payroll settings to a map
		for _, p := range payCalendarResp.PayrollCalendars {
			payrollCalendarMap[p.PayrollCalendarID] = p.PaymentDate
		}

		for empName, leaveReq := range leaveRequests[orgName] {
			errStr := service.processLeaveRequestByEmp(ctx, xeroEmployeesMap, empName, leaveReq, tenantID, payrollCalendarMap, errChan, &wg)
			errStrings = append(errStrings, errStr)
		}
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	fmt.Println(errStrings)
	for _, e := range errStrings {
		if e.Error() != "" {
			errResult = append(errResult, e.Error())
		}
	}
	for err := range errChan {
		if err != nil {
			errResult = append(errResult, err.Error())
		}
	}
	if len(errResult) > 0 {
		return errResult
	}
	return nil
}

func (service Service) processLeaveRequestByEmp(ctx context.Context, xeroEmployeesMap map[string]xero.Employee,
	empName string, leaveReq []model.KrowLeaveRequest, tenantID string, payrollCalendarMap map[string]string,
	errCh chan error, wg *sync.WaitGroup) error {
	ctxLogger := log.WithContext(ctx)

	if _, ok := xeroEmployeesMap[empName]; !ok {
		errStr := fmt.Errorf("employee not found in Xero: %v", empName)
		ctxLogger.Infof(errStr.Error())
		return errStr
	}
	empID := xeroEmployeesMap[empName].EmployeeID
	payCalendarID := xeroEmployeesMap[empName].PayrollCalendarID
	if _, ok := payrollCalendarMap[payCalendarID]; !ok {
		errStr := fmt.Errorf("Failed to fetch employee payroll calendar settings from Xero. Employee %v ", empName)
		ctxLogger.Infof(errStr.Error())
		return errStr
	}
	paymentDate := payrollCalendarMap[payCalendarID]
	leaveBalance, err := service.client.EmployeeLeaveBalance(ctx, tenantID, empID)
	if err != nil {
		errStr := fmt.Errorf("failed to fetch employee leave balance from Xero. Emp Name %v", empName)
		ctxLogger.Infof(errStr.Error(), err)
		return errStr
	}
	err = service.reconcileLeaveRequestAndApply(ctx, empID, empName, tenantID, leaveReq, paymentDate, leaveBalance, errCh, wg)
	return err
}

func (service Service) reconcileLeaveRequestAndApply(ctx context.Context, empID string, empName string, tenantID string,
	leaveReq []model.KrowLeaveRequest, paymentDate string, leaveBalance *xero.LeaveBalanceResponse, errCh chan error, wg *sync.WaitGroup) error {
	var leaveBalanceMap = make(map[string]xero.LeaveBalance)
	var leaveTypeID string
	var leaveStartDate string
	var leaveEndDate string
	var unpaidLeaveUnits float64
	var leaveUnits float64
	var unPaidLeaveTypeID string
	var errorsStr []string

	ctxLogger := log.WithContext(ctx)
	ctxLogger.Infof("Calculating leaves to be applied for Employee %v", empName)

	for _, leaveBal := range leaveBalance.Employees[0].LeaveBalance {
		leaveBalanceMap[leaveBal.LeaveType] = leaveBal
		if strings.EqualFold(leaveBal.LeaveType, unPaidLeave) {
			unPaidLeaveTypeID = leaveBal.LeaveTypeID
		}
	}

	for _, leave := range leaveReq {
		if _, ok := leaveBalanceMap[leave.LeaveType]; !ok {
			errStr := fmt.Errorf("leave type %v not found in Xero for Employee: %v", leave.LeaveType, empName)
			ctxLogger.Infof(errStr.Error())
			errorsStr = append(errorsStr, errStr.Error())
			continue
		}
		lb := leaveBalanceMap[leave.LeaveType]
		leaveReqUnit := leave.Hours
		availableLeaveBalUnit := lb.NumberOfUnits
		leaveTypeID = lb.LeaveTypeID
		leaveStartDate = "/Date(" + strconv.FormatInt(leave.LeaveDate, 10) + ")/"
		leaveEndDate = "/Date(" + strconv.FormatInt(leave.LeaveDate, 10) + ")/"
		if leaveReqUnit >= availableLeaveBalUnit {
			if availableLeaveBalUnit > 0 {
				leaveUnits = availableLeaveBalUnit
				unpaidLeaveUnits += leaveReqUnit - availableLeaveBalUnit
			} else {
				//Employees has negative or zero leave balance and hence unpaid leave
				leaveUnits = 0
				unpaidLeaveUnits += leaveReqUnit
			}
		} else {
			leaveUnits = leaveReqUnit
		}
		updatedLeaveBalance := xero.LeaveBalance{
			LeaveType:     leave.LeaveType,
			LeaveTypeID:   leaveTypeID,
			NumberOfUnits: lb.NumberOfUnits - leaveUnits,
			TypeOfUnits:   lb.TypeOfUnits,
		}
		leaveBalanceMap[leave.LeaveType] = updatedLeaveBalance

		if leaveUnits > 0 {
			wg.Add(1)
			paidLeaveReq := EmpLeaveRequest{
				empID:          empID,
				empName:        empName,
				tenantID:       tenantID,
				leaveTypeID:    leaveTypeID,
				leaveUnits:     leaveUnits,
				paymentDate:    paymentDate,
				leaveStartDate: leaveStartDate,
				leaveEndDate:   leaveEndDate,
				leaveType:      leave.LeaveType,
			}
			service.applyLeave(ctx, paidLeaveReq, errCh, wg)
		}
	}
	if unpaidLeaveUnits > 0 {
		wg.Add(1)
		unPaidLeaveReq := EmpLeaveRequest{
			empID:          empID,
			empName:        empName,
			tenantID:       tenantID,
			leaveTypeID:    unPaidLeaveTypeID,
			leaveUnits:     unpaidLeaveUnits,
			paymentDate:    paymentDate,
			leaveStartDate: leaveStartDate,
			leaveEndDate:   leaveEndDate,
			leaveType:      unPaidLeave,
		}
		service.applyLeave(ctx, unPaidLeaveReq, errCh, wg)
	}
	e := strings.Join(errorsStr, "\n")
	errRes := errors.New(e)
	return errRes
}

func (service Service) applyLeave(ctx context.Context, leaveReq EmpLeaveRequest, errCh chan error, wg *sync.WaitGroup) {
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
	go service.applyLeaveRequestToXero(ctx, leaveReq.tenantID, leaveApplication, leaveReq.empName, errCh, wg)
}

func (service Service) applyLeaveRequestToXero(ctx context.Context, tenantID string, leaveApplication xero.LeaveApplicationRequest,
	empName string, errCh chan error, wg *sync.WaitGroup) {
	ctxLogger := log.WithContext(ctx)
	ctxLogger.Infof("Applying leave request for Employees: %v", empName)

	defer wg.Done()
	err := service.client.EmployeeLeaveApplication(ctx, tenantID, leaveApplication)
	if err != nil {
		ctxLogger.Infof("Leave Application Request: %v", leaveApplication)
		ctxLogger.WithError(err).Errorf("Failed to post Leave application to xero for employee %v", empName)
		errCh <- fmt.Errorf("failed to post Leave application to xero for employee %v. Check if Unpaid Leave is configured", empName)
	}
	errCh <- nil
}

func (service Service) extractDataFromKrow(ctx context.Context) (map[string]map[string][]model.KrowLeaveRequest, error) {
	var leaveRequests = make(map[string]map[string][]model.KrowLeaveRequest)
	var digIOLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var eliizaLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var cmdLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var kasnaLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var mantelLeaveReq = make(map[string][]model.KrowLeaveRequest)
	ctxLogger := log.WithContext(ctx)

	f, err := excelize.OpenFile(service.xlsFileLocation)
	if err != nil {
		errStr := fmt.Errorf("unable to open the xls file provided")
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
			errStr := fmt.Errorf("error while parsing leave date for entry: %v", row[1])
			ctxLogger.WithError(err).Error(errStr)
			return nil, errStr
		}
		hours, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			errStr := fmt.Errorf("error while parsing leave hours for entry: %v", row[2])
			ctxLogger.WithError(err).Error(errStr)
			return nil, errStr
		}
		leaveType := row[5]
		if leaveType == "" {
			leaveType = row[3]
		}
		r := strings.NewReplacer("Carers", "Carer's",
			"Unpaid", "Other Unpaid")
		leaveType = r.Replace(leaveType)
		empName := row[0]
		orgName := row[6]
		leaveReq := model.KrowLeaveRequest{
			LeaveDate: leaveDate.UnixNano() / 1000000,
			Hours:     hours,
			LeaveType: leaveType,
			OrgName:   orgName,
		}
		switch orgName {
		case digIO:
			digIOLeaveReq[empName] = append(digIOLeaveReq[empName], leaveReq)
			break
		case cmd:
			cmdLeaveReq[empName] = append(cmdLeaveReq[empName], leaveReq)
			break
		case eliiza:
			eliizaLeaveReq[empName] = append(eliizaLeaveReq[empName], leaveReq)
			break
		case kasna:
			kasnaLeaveReq[empName] = append(kasnaLeaveReq[empName], leaveReq)
			break
		case mantel:
			mantelLeaveReq[empName] = append(mantelLeaveReq[empName], leaveReq)
			break
		}
	}

	leaveRequests[digIO] = digIOLeaveReq
	leaveRequests[cmd] = cmdLeaveReq
	leaveRequests[eliiza] = eliizaLeaveReq
	leaveRequests[kasna] = kasnaLeaveReq
	leaveRequests[mantel] = mantelLeaveReq

	return leaveRequests, nil
}
