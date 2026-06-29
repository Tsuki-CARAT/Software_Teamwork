package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
	"github.com/jackc/pgx/v5"
)

// --- Reports ---

func (r *PostgresRepository) ListReports(ctx context.Context, filter service.ReportListFilter) ([]service.Report, int, error) {
	conditions := []string{"1 = 1"}
	args := []any{}
	argN := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	if strings.TrimSpace(filter.CreatorID) != "" {
		conditions = append(conditions, "creator_id = "+argN(filter.CreatorID))
	}
	if strings.TrimSpace(filter.ReportType) != "" {
		conditions = append(conditions, "report_type = "+argN(filter.ReportType))
	}
	if strings.TrimSpace(filter.Status) != "" {
		conditions = append(conditions, "status = "+argN(filter.Status))
	} else {
		conditions = append(conditions, "status <> 'deleted'")
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		conditions = append(conditions, "report_name ILIKE "+argN("%"+filter.Keyword+"%"))
	}

	where := strings.Join(conditions, " AND ")

	var total int
	countRow := r.db.QueryRow(ctx, "SELECT count(*) FROM reports WHERE "+where, args...)
	if err := countRow.Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	limitArg := argN(pageSize)
	offsetArg := argN((page - 1) * pageSize)

	query := fmt.Sprintf(`
		SELECT
			id::text, report_name, report_type, COALESCE(template_id::text, ''), topic,
			COALESCE(specialty, ''), COALESCE(plant_or_business_object, ''),
			COALESCE(report_year, 0), status, COALESCE(creator_id, ''),
			COALESCE(creator_name, ''), source, COALESCE(latest_job_id::text, ''),
			COALESCE(latest_report_file_id::text, ''), generated_at, exported_at,
			created_at, updated_at, deleted_at
		FROM reports
		WHERE %s
		ORDER BY created_at DESC
		LIMIT %s OFFSET %s`, where, limitArg, offsetArg)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	reports := make([]service.Report, 0)
	for rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan report: %w", err)
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate reports: %w", err)
	}
	return reports, total, nil
}

func (r *PostgresRepository) GetReportByID(ctx context.Context, id string) (service.Report, error) {
	reportID, err := parseUUID(id)
	if err != nil {
		return service.Report{}, service.NewError(service.CodeValidation, "invalid report id", err)
	}
	row := r.db.QueryRow(ctx, `
		SELECT
			id::text, report_name, report_type, COALESCE(template_id::text, ''), topic,
			COALESCE(specialty, ''), COALESCE(plant_or_business_object, ''),
			COALESCE(report_year, 0), status, COALESCE(creator_id, ''),
			COALESCE(creator_name, ''), source, COALESCE(latest_job_id::text, ''),
			COALESCE(latest_report_file_id::text, ''), generated_at, exported_at,
			created_at, updated_at, deleted_at
		FROM reports
		WHERE id = $1`, reportID)
	report, err := scanReport(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.Report{}, service.NewError(service.CodeNotFound, "report not found", err)
		}
		return service.Report{}, fmt.Errorf("get report: %w", err)
	}
	return report, nil
}

func (r *PostgresRepository) UpdateReport(ctx context.Context, value service.Report) (service.Report, error) {
	reportID, err := parseUUID(value.ID)
	if err != nil {
		return service.Report{}, service.NewError(service.CodeValidation, "invalid report id", err)
	}
	row := r.db.QueryRow(ctx, `
		UPDATE reports SET
			report_name = $2,
			template_id = NULLIF($3, '')::uuid,
			topic = $4,
			specialty = NULLIF($5, ''),
			plant_or_business_object = NULLIF($6, ''),
			report_year = NULLIF($7, 0),
			updated_at = $8
		WHERE id = $1
		RETURNING
			id::text, report_name, report_type, COALESCE(template_id::text, ''), topic,
			COALESCE(specialty, ''), COALESCE(plant_or_business_object, ''),
			COALESCE(report_year, 0), status, COALESCE(creator_id, ''),
			COALESCE(creator_name, ''), source, COALESCE(latest_job_id::text, ''),
			COALESCE(latest_report_file_id::text, ''), generated_at, exported_at,
			created_at, updated_at, deleted_at`,
		reportID,
		value.Name,
		value.TemplateID,
		value.Topic,
		value.Specialty,
		value.BusinessObject,
		value.Year,
		value.UpdatedAt,
	)
	report, err := scanReport(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.Report{}, service.NewError(service.CodeNotFound, "report not found", err)
		}
		return service.Report{}, fmt.Errorf("update report: %w", err)
	}
	return report, nil
}

func (r *PostgresRepository) SoftDeleteReport(ctx context.Context, id string, deletedAt time.Time) (service.Report, error) {
	reportID, err := parseUUID(id)
	if err != nil {
		return service.Report{}, service.NewError(service.CodeValidation, "invalid report id", err)
	}
	row := r.db.QueryRow(ctx, `
		UPDATE reports SET status = 'deleted', deleted_at = $2, updated_at = $2
		WHERE id = $1
		RETURNING
			id::text, report_name, report_type, COALESCE(template_id::text, ''), topic,
			COALESCE(specialty, ''), COALESCE(plant_or_business_object, ''),
			COALESCE(report_year, 0), status, COALESCE(creator_id, ''),
			COALESCE(creator_name, ''), source, COALESCE(latest_job_id::text, ''),
			COALESCE(latest_report_file_id::text, ''), generated_at, exported_at,
			created_at, updated_at, deleted_at`,
		reportID, deletedAt,
	)
	report, err := scanReport(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.Report{}, service.NewError(service.CodeNotFound, "report not found", err)
		}
		return service.Report{}, fmt.Errorf("soft delete report: %w", err)
	}
	return report, nil
}

// --- Outlines ---

func (r *PostgresRepository) CreateReportOutline(ctx context.Context, value service.ReportOutline) (service.ReportOutline, error) {
	reportID, err := parseUUID(value.ReportID)
	if err != nil {
		return service.ReportOutline{}, service.NewError(service.CodeValidation, "invalid report id", err)
	}
	sectionsJSON, err := json.Marshal(value.Sections)
	if err != nil {
		return service.ReportOutline{}, fmt.Errorf("marshal outline sections: %w", err)
	}

	if value.IsCurrent {
		if _, err := r.db.Exec(ctx, `UPDATE report_outlines SET is_current = false WHERE report_id = $1`, reportID); err != nil {
			return service.ReportOutline{}, fmt.Errorf("unset previous current outline: %w", err)
		}
	}

	row := r.db.QueryRow(ctx, `
		INSERT INTO report_outlines (
			id, report_id, outline_json, version, source, source_job_id,
			is_current, manual_edited, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::uuid, $7, $8, $9, $10)
		RETURNING
			id::text, report_id::text, outline_json, version, source,
			COALESCE(source_job_id::text, ''), is_current, manual_edited, created_at, updated_at`,
		value.ID,
		reportID,
		sectionsJSON,
		value.Version,
		string(value.Source),
		value.SourceJobID,
		value.IsCurrent,
		value.ManualEdited,
		value.CreatedAt,
		value.UpdatedAt,
	)
	outline, err := scanReportOutline(row)
	if err != nil {
		if isUniqueViolation(err) {
			return service.ReportOutline{}, service.NewError(service.CodeConflict, "report outline version already exists", err)
		}
		return service.ReportOutline{}, fmt.Errorf("insert report outline: %w", err)
	}
	return outline, nil
}

func (r *PostgresRepository) ListReportOutlines(ctx context.Context, reportID string) ([]service.ReportOutline, error) {
	id, err := parseUUID(reportID)
	if err != nil {
		return nil, service.NewError(service.CodeValidation, "invalid report id", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT
			id::text, report_id::text, outline_json, version, source,
			COALESCE(source_job_id::text, ''), is_current, manual_edited, created_at, updated_at
		FROM report_outlines
		WHERE report_id = $1
		ORDER BY version DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("list report outlines: %w", err)
	}
	defer rows.Close()

	outlines := make([]service.ReportOutline, 0)
	for rows.Next() {
		outline, err := scanReportOutline(rows)
		if err != nil {
			return nil, fmt.Errorf("scan report outline: %w", err)
		}
		outlines = append(outlines, outline)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report outlines: %w", err)
	}
	return outlines, nil
}

func (r *PostgresRepository) GetReportOutlineByID(ctx context.Context, id string) (service.ReportOutline, error) {
	outlineID, err := parseUUID(id)
	if err != nil {
		return service.ReportOutline{}, service.NewError(service.CodeValidation, "invalid report outline id", err)
	}
	row := r.db.QueryRow(ctx, `
		SELECT
			id::text, report_id::text, outline_json, version, source,
			COALESCE(source_job_id::text, ''), is_current, manual_edited, created_at, updated_at
		FROM report_outlines
		WHERE id = $1`, outlineID)
	outline, err := scanReportOutline(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ReportOutline{}, service.NewError(service.CodeNotFound, "report outline not found", err)
		}
		return service.ReportOutline{}, fmt.Errorf("get report outline: %w", err)
	}
	return outline, nil
}

func (r *PostgresRepository) UpdateReportOutline(ctx context.Context, value service.ReportOutline) (service.ReportOutline, error) {
	outlineID, err := parseUUID(value.ID)
	if err != nil {
		return service.ReportOutline{}, service.NewError(service.CodeValidation, "invalid report outline id", err)
	}
	sectionsJSON, err := json.Marshal(value.Sections)
	if err != nil {
		return service.ReportOutline{}, fmt.Errorf("marshal outline sections: %w", err)
	}
	row := r.db.QueryRow(ctx, `
		UPDATE report_outlines SET
			outline_json = $2,
			manual_edited = $3,
			updated_at = $4
		WHERE id = $1
		RETURNING
			id::text, report_id::text, outline_json, version, source,
			COALESCE(source_job_id::text, ''), is_current, manual_edited, created_at, updated_at`,
		outlineID,
		sectionsJSON,
		value.ManualEdited,
		value.UpdatedAt,
	)
	outline, err := scanReportOutline(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ReportOutline{}, service.NewError(service.CodeNotFound, "report outline not found", err)
		}
		return service.ReportOutline{}, fmt.Errorf("update report outline: %w", err)
	}
	return outline, nil
}

// --- Sections ---

func (r *PostgresRepository) CreateReportSection(ctx context.Context, value service.ReportSection) (service.ReportSection, error) {
	reportID, err := parseUUID(value.ReportID)
	if err != nil {
		return service.ReportSection{}, service.NewError(service.CodeValidation, "invalid report id", err)
	}
	tablesJSON, err := marshalTables(value.Tables)
	if err != nil {
		return service.ReportSection{}, err
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO report_sections (
			id, report_id, outline_id, parent_id, outline_node_id, section_path,
			title, level, sort_order, numbering, section_type, content, tables_json,
			generation_status, content_source, manual_edited, version, last_job_id,
			created_at, updated_at
		)
		VALUES (
			$1, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, NULLIF($5, ''), $6,
			$7, $8, $9, NULLIF($10, ''), $11, $12, $13,
			$14, NULLIF($15, ''), $16, $17, NULLIF($18, '')::uuid,
			$19, $20
		)
		RETURNING
			id::text, report_id::text, COALESCE(outline_id::text, ''), COALESCE(parent_id::text, ''),
			COALESCE(outline_node_id, ''), section_path, title, level, sort_order,
			COALESCE(numbering, ''), section_type, content, tables_json,
			generation_status, COALESCE(content_source, ''), manual_edited, version,
			COALESCE(last_job_id::text, ''), generated_at, created_at, updated_at`,
		value.ID,
		reportID,
		value.OutlineID,
		value.ParentID,
		value.OutlineNodeID,
		value.SectionPath,
		value.Title,
		value.Level,
		value.SortOrder,
		value.Numbering,
		string(value.SectionType),
		value.Content,
		tablesJSON,
		string(value.GenerationStatus),
		string(value.ContentSource),
		value.ManualEdited,
		value.Version,
		value.LastJobID,
		value.CreatedAt,
		value.UpdatedAt,
	)
	section, err := scanReportSection(row)
	if err != nil {
		if isUniqueViolation(err) {
			return service.ReportSection{}, service.NewError(service.CodeConflict, "report section already exists", err)
		}
		return service.ReportSection{}, fmt.Errorf("insert report section: %w", err)
	}
	return section, nil
}

func (r *PostgresRepository) ListReportSections(ctx context.Context, reportID string) ([]service.ReportSection, error) {
	id, err := parseUUID(reportID)
	if err != nil {
		return nil, service.NewError(service.CodeValidation, "invalid report id", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT
			id::text, report_id::text, COALESCE(outline_id::text, ''), COALESCE(parent_id::text, ''),
			COALESCE(outline_node_id, ''), section_path, title, level, sort_order,
			COALESCE(numbering, ''), section_type, content, tables_json,
			generation_status, COALESCE(content_source, ''), manual_edited, version,
			COALESCE(last_job_id::text, ''), generated_at, created_at, updated_at
		FROM report_sections
		WHERE report_id = $1
		ORDER BY sort_order ASC, created_at ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("list report sections: %w", err)
	}
	defer rows.Close()

	sections := make([]service.ReportSection, 0)
	for rows.Next() {
		section, err := scanReportSection(rows)
		if err != nil {
			return nil, fmt.Errorf("scan report section: %w", err)
		}
		sections = append(sections, section)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report sections: %w", err)
	}
	return sections, nil
}

func (r *PostgresRepository) GetReportSectionByID(ctx context.Context, id string) (service.ReportSection, error) {
	sectionID, err := parseUUID(id)
	if err != nil {
		return service.ReportSection{}, service.NewError(service.CodeValidation, "invalid report section id", err)
	}
	row := r.db.QueryRow(ctx, `
		SELECT
			id::text, report_id::text, COALESCE(outline_id::text, ''), COALESCE(parent_id::text, ''),
			COALESCE(outline_node_id, ''), section_path, title, level, sort_order,
			COALESCE(numbering, ''), section_type, content, tables_json,
			generation_status, COALESCE(content_source, ''), manual_edited, version,
			COALESCE(last_job_id::text, ''), generated_at, created_at, updated_at
		FROM report_sections
		WHERE id = $1`, sectionID)
	section, err := scanReportSection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ReportSection{}, service.NewError(service.CodeNotFound, "report section not found", err)
		}
		return service.ReportSection{}, fmt.Errorf("get report section: %w", err)
	}
	return section, nil
}

func (r *PostgresRepository) UpdateReportSection(ctx context.Context, value service.ReportSection) (service.ReportSection, error) {
	sectionID, err := parseUUID(value.ID)
	if err != nil {
		return service.ReportSection{}, service.NewError(service.CodeValidation, "invalid report section id", err)
	}
	tablesJSON, err := marshalTables(value.Tables)
	if err != nil {
		return service.ReportSection{}, err
	}
	row := r.db.QueryRow(ctx, `
		UPDATE report_sections SET
			title = $2,
			content = $3,
			tables_json = $4,
			content_source = NULLIF($5, ''),
			manual_edited = $6,
			version = $7,
			updated_at = $8
		WHERE id = $1
		RETURNING
			id::text, report_id::text, COALESCE(outline_id::text, ''), COALESCE(parent_id::text, ''),
			COALESCE(outline_node_id, ''), section_path, title, level, sort_order,
			COALESCE(numbering, ''), section_type, content, tables_json,
			generation_status, COALESCE(content_source, ''), manual_edited, version,
			COALESCE(last_job_id::text, ''), generated_at, created_at, updated_at`,
		sectionID,
		value.Title,
		value.Content,
		tablesJSON,
		string(value.ContentSource),
		value.ManualEdited,
		value.Version,
		value.UpdatedAt,
	)
	section, err := scanReportSection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ReportSection{}, service.NewError(service.CodeNotFound, "report section not found", err)
		}
		return service.ReportSection{}, fmt.Errorf("update report section: %w", err)
	}
	return section, nil
}

// --- Section versions ---

func (r *PostgresRepository) CreateReportSectionVersion(ctx context.Context, value service.ReportSectionVersion) (service.ReportSectionVersion, error) {
	reportID, err := parseUUID(value.ReportID)
	if err != nil {
		return service.ReportSectionVersion{}, service.NewError(service.CodeValidation, "invalid report id", err)
	}
	sectionID, err := parseUUID(value.SectionID)
	if err != nil {
		return service.ReportSectionVersion{}, service.NewError(service.CodeValidation, "invalid report section id", err)
	}
	tablesJSON, err := marshalTables(value.Tables)
	if err != nil {
		return service.ReportSectionVersion{}, err
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO report_section_versions (
			id, report_id, section_id, version, source, content, tables_json,
			job_id, requirements, created_by, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, '')::uuid, NULLIF($9, ''), NULLIF($10, ''), $11)
		RETURNING
			id::text, report_id::text, section_id::text, version, source, content,
			tables_json, COALESCE(job_id::text, ''), COALESCE(requirements, ''),
			COALESCE(created_by, ''), created_at`,
		value.ID,
		reportID,
		sectionID,
		value.Version,
		string(value.Source),
		value.Content,
		tablesJSON,
		value.JobID,
		value.Requirements,
		value.CreatedBy,
		value.CreatedAt,
	)
	version, err := scanReportSectionVersion(row)
	if err != nil {
		if isUniqueViolation(err) {
			return service.ReportSectionVersion{}, service.NewError(service.CodeConflict, "report section version already exists", err)
		}
		return service.ReportSectionVersion{}, fmt.Errorf("insert report section version: %w", err)
	}
	return version, nil
}

func (r *PostgresRepository) ListReportSectionVersions(ctx context.Context, sectionID string) ([]service.ReportSectionVersion, error) {
	id, err := parseUUID(sectionID)
	if err != nil {
		return nil, service.NewError(service.CodeValidation, "invalid report section id", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT
			id::text, report_id::text, section_id::text, version, source, content,
			tables_json, COALESCE(job_id::text, ''), COALESCE(requirements, ''),
			COALESCE(created_by, ''), created_at
		FROM report_section_versions
		WHERE section_id = $1
		ORDER BY version DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("list report section versions: %w", err)
	}
	defer rows.Close()

	versions := make([]service.ReportSectionVersion, 0)
	for rows.Next() {
		version, err := scanReportSectionVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan report section version: %w", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report section versions: %w", err)
	}
	return versions, nil
}

// --- scanning helpers ---

func scanReportOutline(row scanner) (service.ReportOutline, error) {
	var value service.ReportOutline
	var outlineJSON []byte
	var source string
	if err := row.Scan(
		&value.ID,
		&value.ReportID,
		&outlineJSON,
		&value.Version,
		&source,
		&value.SourceJobID,
		&value.IsCurrent,
		&value.ManualEdited,
		&value.CreatedAt,
		&value.UpdatedAt,
	); err != nil {
		return service.ReportOutline{}, err
	}
	value.Source = service.OutlineSource(source)
	if len(outlineJSON) > 0 {
		if err := json.Unmarshal(outlineJSON, &value.Sections); err != nil {
			return service.ReportOutline{}, fmt.Errorf("unmarshal outline sections: %w", err)
		}
	}
	return value, nil
}

func scanReportSection(row scanner) (service.ReportSection, error) {
	var value service.ReportSection
	var tablesJSON []byte
	var sectionType, generationStatus, contentSource string
	if err := row.Scan(
		&value.ID,
		&value.ReportID,
		&value.OutlineID,
		&value.ParentID,
		&value.OutlineNodeID,
		&value.SectionPath,
		&value.Title,
		&value.Level,
		&value.SortOrder,
		&value.Numbering,
		&sectionType,
		&value.Content,
		&tablesJSON,
		&generationStatus,
		&contentSource,
		&value.ManualEdited,
		&value.Version,
		&value.LastJobID,
		&value.GeneratedAt,
		&value.CreatedAt,
		&value.UpdatedAt,
	); err != nil {
		return service.ReportSection{}, err
	}
	value.SectionType = service.SectionType(sectionType)
	value.GenerationStatus = service.JobStatus(generationStatus)
	value.ContentSource = service.ContentSource(contentSource)
	if len(tablesJSON) > 0 {
		if err := json.Unmarshal(tablesJSON, &value.Tables); err != nil {
			return service.ReportSection{}, fmt.Errorf("unmarshal section tables: %w", err)
		}
	}
	return value, nil
}

func scanReportSectionVersion(row scanner) (service.ReportSectionVersion, error) {
	var value service.ReportSectionVersion
	var tablesJSON []byte
	var source string
	if err := row.Scan(
		&value.ID,
		&value.ReportID,
		&value.SectionID,
		&value.Version,
		&source,
		&value.Content,
		&tablesJSON,
		&value.JobID,
		&value.Requirements,
		&value.CreatedBy,
		&value.CreatedAt,
	); err != nil {
		return service.ReportSectionVersion{}, err
	}
	value.Source = service.ContentSource(source)
	if len(tablesJSON) > 0 {
		if err := json.Unmarshal(tablesJSON, &value.Tables); err != nil {
			return service.ReportSectionVersion{}, fmt.Errorf("unmarshal section version tables: %w", err)
		}
	}
	return value, nil
}

func marshalTables(tables []map[string]any) ([]byte, error) {
	if tables == nil {
		return []byte("[]"), nil
	}
	data, err := json.Marshal(tables)
	if err != nil {
		return nil, fmt.Errorf("marshal section tables: %w", err)
	}
	return data, nil
}
