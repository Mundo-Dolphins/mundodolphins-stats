package audit

// AuditReport holds the complete audit information for a season.
type AuditReport struct {
	Year        int                  `json:"year"`
	TablesFound map[string]TableInfo `json:"tables_found"`
}

// BuildReport creates an audit report from the processed season inventory.
func BuildReport(inv SeasonInventory) AuditReport {
	return AuditReport{
		Year:        inv.Year,
		TablesFound: inv.Tables,
	}
}
