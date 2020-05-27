package internal

import (
	"context"
	"github.com/360EntSecGroup-Skylar/excelize/v2"
	log "github.com/sirupsen/logrus"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/model"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/xero"
	"strconv"
	"strings"
	"time"
)

const digIO = "DigIO"
const cmd = "CMD"
const eliiza = "Eliiza"
const kasna = "Kasna"
const mantel = "Mantel Group"
const unPaidLeave = "Other Unpaid Leave"
const xlsFileLocation = "/Users/syril/sample.xlsx"

type Service struct {
	client xero.ClientInterface
}

func NewService(c xero.ClientInterface) *Service {
	return &Service{
		client: c,
	}
}

func (service Service) MigrateLeaveKrowToXero(ctx context.Context) (model.XeroEmployees, error) {
	var connections []model.Connection
	var employees []model.Employee
	var xeroEmployeesMap map[string]xero.Employee
	var payrollCalendarMap = make(map[string]string)
	var leaveReqChan = make(chan map[string]map[string][]model.KrowLeaveRequest)

	ctxLogger := log.WithContext(ctx)
	ctxLogger.Infof("Executing MigrateLeaveKrowToXero service")

	go service.extractDataFromKrow(ctx, leaveReqChan)

	resp, err := service.client.GetConnections(ctx)
	if err != nil {
		ctxLogger.Infof("Failed to fetch connections from Xero: %v", err)
		return nil, err
	}

	leaveRequests := <-leaveReqChan

	for _, c := range *resp {
		xeroEmployeesMap = make(map[string]xero.Employee)
		connection := model.Connection{
			TenantID:   c.TenantID,
			TenantType: c.TenantType,
			OrgName:    c.OrgName,
		}
		empResponse, err := service.client.GetEmployees(ctx, connection.TenantID)
		if err != nil {
			ctxLogger.Infof("Failed to fetch employees from Xero: %v", err)
			return nil, err
		}
		for _, emp := range empResponse.Employees {
			xeroEmployeesMap[emp.FirstName+" "+emp.LastName] = emp
		}

		payCalendarResp, err := service.client.GetPayrollCalendars(ctx, connection.TenantID)
		if err != nil {
			ctxLogger.Infof("Failed to fetch employee payroll calendar settings from Xero: %v", err)
		}

		for _, p := range payCalendarResp.PayrollCalendars {
			payrollCalendarMap[p.PayrollCalendarID] = p.PaymentDate
		}

		for empName, leaveReq := range leaveRequests[connection.OrgName] {
			service.processLeaveRequestByEmp(ctx, xeroEmployeesMap, empName, leaveReq, connection.TenantID, payrollCalendarMap)
		}
		connections = append(connections, connection)
	}

	return employees, nil
}

func (service Service) processLeaveRequestByEmp(ctx context.Context, xeroEmployeesMap map[string]xero.Employee,
	empName string, leaveReq []model.KrowLeaveRequest, tenantID string, payrollCalendarMap map[string]string) {
	ctxLogger := log.WithContext(ctx)
	if _, ok := xeroEmployeesMap[empName]; ok {
		empID := xeroEmployeesMap[empName].EmployeeID
		payCalendarID := xeroEmployeesMap[empName].PayrollCalendarID
		if paymentDate, ok := payrollCalendarMap[payCalendarID]; ok {
			leaveBalance, err := service.client.EmployeeLeaveBalance(ctx, tenantID, empID)
			if err != nil {
				ctxLogger.Infof("Failed to fetch employee leave balance from Xero: %v", err)
			}
			go service.reconcileLeaveRequestAndApply(ctx, empID, empName, tenantID, leaveReq, paymentDate, leaveBalance)
		} else {
			ctxLogger.Infof("Failed to fetch employee payroll calendar settings from Xero. Employee %v ", empName)
		}
	} else {
		ctxLogger.Infof("Employee not found in Xero: %v", empName)
	}
}

func (service Service) reconcileLeaveRequestAndApply(ctx context.Context, empID string, empName string, tenantID string,
	leaveReq []model.KrowLeaveRequest, paymentDate string, leaveBalance *xero.LeaveBalanceResponse) {
	var leaveBalanceMap = make(map[string]xero.LeaveBalance)
	var leaveTypeID string
	var leaveStartDate string
	var leaveEndDate string
	var unpaidLeave float64
	var leaveUnits float64
	var unPaidLeaveTypeID string

	ctxLogger := log.WithContext(ctx)
	ctxLogger.Infof("Calculating leaves to be applied for Employee %v", empName)
	for _, leaveBal := range leaveBalance.Employee[0].LeaveBalance {
		leaveBalanceMap[leaveBal.LeaveType] = leaveBal
		if strings.EqualFold(leaveBal.LeaveType, unPaidLeave) {
			unPaidLeaveTypeID = leaveBal.LeaveTypeID
		}
	}

	for _, leave := range leaveReq {
		var leavePeriods = make([]xero.LeavePeriod, 1)
		if lb, ok := leaveBalanceMap[leave.LeaveType]; ok {
			leaveReqUnit := leave.Hours
			availableLeaveBalUnit := lb.NumberOfUnits
			leaveTypeID = lb.LeaveTypeID
			leaveStartDate = "/Date(" + strconv.FormatInt(leave.LeaveDate, 10) + ")/"
			leaveEndDate = "/Date(" + strconv.FormatInt(leave.LeaveDate, 10) + ")/"
			if leaveReqUnit >= availableLeaveBalUnit {
				if availableLeaveBalUnit > 0 {
					leaveUnits = availableLeaveBalUnit
					unpaidLeave += leaveReqUnit - availableLeaveBalUnit
				} else {
					//Employee has negative or zero leave balance and hence unpaid leave
					leaveUnits = 0
					unpaidLeave += leaveReqUnit
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
				paidLeave := xero.LeavePeriod{
					PayPeriodEndDate: paymentDate,
					NumberOfUnits:    leaveUnits,
				}

				leavePeriods[0] = paidLeave
				leaveApplication := xero.LeaveApplicationRequest{
					EmployeeID:   empID,
					LeaveTypeID:  leaveTypeID,
					StartDate:    leaveStartDate,
					EndDate:      leaveEndDate,
					Title:        leave.LeaveType,
					LeavePeriods: leavePeriods,
				}
				go service.applyLeaveRequestToXero(ctx, tenantID, leaveApplication, empName)
			}
		} else {
			ctxLogger.Infof("Leave type not found in Xero: %v", leave.LeaveType)
		}
	}
	if unpaidLeave > 0 {
		unpaidLeavePeriod := make([]xero.LeavePeriod, 1)
		unpaidLeaveReq := xero.LeavePeriod{
			PayPeriodEndDate: paymentDate,
			NumberOfUnits:    unpaidLeave,
		}
		unpaidLeavePeriod[0] = unpaidLeaveReq

		unpaidLeaveApplication := xero.LeaveApplicationRequest{
			EmployeeID:   empID,
			LeaveTypeID:  unPaidLeaveTypeID,
			StartDate:    leaveStartDate,
			EndDate:      leaveEndDate,
			Title:        unPaidLeave,
			LeavePeriods: unpaidLeavePeriod,
		}
		go service.applyLeaveRequestToXero(ctx, tenantID, unpaidLeaveApplication, empName)
	}
}

func (service Service) applyLeaveRequestToXero(ctx context.Context, tenantID string, leaveApplication xero.LeaveApplicationRequest, empName string) {
	ctxLogger := log.WithContext(ctx)
	ctxLogger.Infof("Applying leave request for Employee: %v", empName)
	err := service.client.EmployeeLeaveApplication(ctx, tenantID, leaveApplication)
	if err != nil {
		ctxLogger.WithError(err).Errorf("failed to post Leave application to xero for employee %v", empName)
	}
}

func (service Service) extractDataFromKrow(ctx context.Context, ch chan map[string]map[string][]model.KrowLeaveRequest) {
	var leaveRequests = make(map[string]map[string][]model.KrowLeaveRequest)
	var digIOLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var eliizaLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var cmdLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var kasnaLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var mantelLeaveReq = make(map[string][]model.KrowLeaveRequest)
	ctxLogger := log.WithContext(ctx)
	f, err := excelize.OpenFile(xlsFileLocation)
	if err != nil {
		ctxLogger.WithError(err).Error("unable to open the xls file provided")
		ch <- leaveRequests
	}

	rows, err := f.GetRows("Sheet1")
	for index, row := range rows {
		if index == 0 {
			continue
		}
		leaveDate, err := time.Parse("2/1/2006", row[1])
		if err != nil {
			ctxLogger.WithError(err).Error("error while parsing leave date")
		}
		hours, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			ctxLogger.WithError(err).Error("error while parsing leave hours")
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

	ch <- leaveRequests
}
