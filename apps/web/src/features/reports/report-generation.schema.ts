import { z } from 'zod'

export const createReportSchema = z.object({
  name: z.string().trim().min(1, '请输入报告名称'),
  reportType: z.string().trim().min(1, '请选择报告类型'),
  templateId: z.string().trim().min(1, '请选择报告模板'),
  topic: z.string().trim().min(1, '请输入报告主题'),
  specialty: z.string().trim().optional(),
  businessObject: z.string().trim().optional(),
  year: z.coerce
    .number()
    .int('年份必须是整数')
    .min(2000, '年份不能早于 2000')
    .max(2100, '年份不能晚于 2100'),
  extraContextText: z.string().trim().optional(),
})

export type CreateReportFormValues = z.infer<typeof createReportSchema>

export const defaultCreateReportValues: CreateReportFormValues = {
  name: '2026年迎峰度夏检查报告',
  reportType: 'summer_peak_inspection',
  templateId: '',
  topic: '迎峰度夏设备安全检查',
  specialty: '电气一次',
  businessObject: '主变、厂用电系统、保护装置',
  year: new Date().getFullYear(),
  extraContextText: '重点关注高温高负荷、缺陷闭环、应急保障和历史隐患治理。',
}
