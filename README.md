# migrate-leaves-to-xero
This is a project to automate the leave migration process i.e. migrating the leaves entered in Krow tool to Xero.

# The Problem
- Leave is entered by team members into krow and on weekly basis the Operations Team have to perform a manual extract of leave and enter into xero.
- If a person has entered leave as annual leave but does not have sufficient balance the operations team have to manually split between the paid and unpaid component
- If the leave spans 2 pay cycles then only the leave from the 1st pay cycle can be entered 
- leave cannot be entered if time sheet not approved

# MVP
- Adjusting leave balance for leave entered
- Changing leave type for when balance is insufficient
- Some type of error report for items not processed (only needs to include the lines)
- Report listing items successfully entered (can be very basic but required for auditing - important that this lists any leave where the category was changed as we need to inform the team member so they aren't surprised come payroll when a week of annual leave was changed to unpaid leave)
