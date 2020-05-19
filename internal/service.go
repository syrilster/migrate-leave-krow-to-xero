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
	var xeroEmployeesMap = make(map[string]xero.Employee)
	//var leaveRequest map[string][]model.KrowLeaveRequest
	var leaveReqChan = make(chan map[string]map[string][]model.KrowLeaveRequest)

	//var empResp *xero.EmpResponse
	ctxLogger := log.WithContext(ctx)
	ctxLogger.Infof("Inside the MigrateLeaveKrowToXero service")

	go extractDataFromKrow(ctx, leaveReqChan)

	resp, err := service.client.GetConnections(ctx)
	if err != nil {
		ctxLogger.Infof("Failed to fetch connections from Xero: %v", err)
		return nil, err
	}

	leaveRequests := <-leaveReqChan

	for _, c := range *resp {
		connection := model.Connection{
			TenantID:   c.TenantID,
			TenantType: c.TenantType,
			OrgName:    c.OrgName,
		}
		empResp, err := service.client.GetEmployees(ctx, connection.TenantID)
		if err != nil {
			ctxLogger.Infof("Failed to fetch employees from Xero: %v", err)
			return nil, err
		}
		for _, emp := range empResp.Employees {
			xeroEmployeesMap[emp.FirstName+" "+emp.LastName] = emp
		}

		for emp, leaveReq := range leaveRequests[connection.OrgName] {
			var leaveBalanceMap = make(map[string]xero.LeaveBalance)
			var leaveTypeID string
			var leaveStartDate string
			var leaveEndDate string
			var unpaidLeave float64
			var leaveUnits float64
			empID := xeroEmployeesMap[emp].EmployeeID
			payCalendarID := xeroEmployeesMap[emp].PayrollCalendarID
			payCalendarResp, err := service.client.GetEmployeePayrollCalendar(ctx, connection.TenantID, payCalendarID)
			if err != nil {
				ctxLogger.Infof("Failed to fetch employee leave balance from Xero: %v", err)
			}
			payPeriodEndDate := payCalendarResp.PayrollCalendars[0].PaymentDate
			leaveBalance, err := service.client.EmployeeLeaveBalance(ctx, connection.TenantID, empID)
			if err != nil {
				ctxLogger.Infof("Failed to fetch employee leave balance from Xero: %v", err)
			}
			for _, leaveBal := range leaveBalance.Employee[0].LeaveBalance {
				leaveBalanceMap[leaveBal.LeaveType] = leaveBal
			}
			for _, leave := range leaveReq {
				var leavePeriods = make([]xero.LeavePeriod, 1)
				if lb, ok := leaveBalanceMap[leave.LeaveType]; ok {
					leaveReqUnit := leave.Hours
					leaveBalUnit := lb.NumberOfUnits
					leaveTypeID = lb.LeaveTypeID
					leaveStartDate = "/Date(" + strconv.FormatInt(leave.LeaveDate, 10) + ")/"
					leaveEndDate = "/Date(" + strconv.FormatInt(leave.LeaveDate, 10) + ")/"
					if leaveReqUnit >= leaveBalUnit {
						if leaveBalUnit > 0 {
							leaveUnits = leaveBalUnit
							unpaidLeave += leaveReqUnit - leaveBalUnit
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
							PayPeriodEndDate: payPeriodEndDate,
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

						err := service.client.EmployeeLeaveApplication(ctx, connection.TenantID, leaveApplication)
						if err != nil {
							ctxLogger.WithError(err).Errorf("failed to post Leave application to xero for employee %v", emp)
						}
					}
				} else {
					ctxLogger.Infof("Leave type not found in Xero: %v", err)
				}
			}
			if unpaidLeave > 0 {
				unpaidLeavePeriod := make([]xero.LeavePeriod, 1)
				unpaidLeaveReq := xero.LeavePeriod{
					PayPeriodEndDate: payPeriodEndDate,
					NumberOfUnits:    unpaidLeave,
				}
				unpaidLeavePeriod[0] = unpaidLeaveReq

				unpaidLeaveApplication := xero.LeaveApplicationRequest{
					EmployeeID:   empID,
					LeaveTypeID:  leaveTypeID,
					StartDate:    leaveStartDate,
					EndDate:      leaveEndDate,
					Title:        unPaidLeave,
					LeavePeriods: unpaidLeavePeriod,
				}

				err = service.client.EmployeeLeaveApplication(ctx, connection.TenantID, unpaidLeaveApplication)
				if err != nil {
					ctxLogger.WithError(err).Errorf("failed to post Leave application to xero for employee %v", emp)
				}
			}
		}
		connections = append(connections, connection)
	}

	return employees, nil
}

func extractDataFromKrow(ctx context.Context, ch chan map[string]map[string][]model.KrowLeaveRequest) {
	var leaveRequests = make(map[string]map[string][]model.KrowLeaveRequest)
	var digioLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var eliizaLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var cmdLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var kasnaLeaveReq = make(map[string][]model.KrowLeaveRequest)
	var mantelLeaveReq = make(map[string][]model.KrowLeaveRequest)
	ctxLogger := log.WithContext(ctx)
	f, err := excelize.OpenFile("/Users/syril/test.xlsx")
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
		r := strings.NewReplacer("Carers", "Carer's")
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
			digioLeaveReq[empName] = append(digioLeaveReq[empName], leaveReq)
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

	leaveRequests[digIO] = digioLeaveReq
	leaveRequests[cmd] = cmdLeaveReq
	leaveRequests[eliiza] = eliizaLeaveReq
	leaveRequests[kasna] = kasnaLeaveReq
	leaveRequests[mantel] = mantelLeaveReq

	ch <- leaveRequests
}
