import { describe, expect, it } from 'vitest'
import { toSystemNavGroups } from './system-navigation'

describe('two-level system navigation', () => {
  it('groups every system module under a stable first-level section', () => {
    const groups = toSystemNavGroups('order center/华东')
    expect(groups.map((group) => group.label)).toEqual(['工作台', '资产管理', '场景管理', '执行中心', '系统管理'])
    expect(groups.flatMap((group) => group.items.map((item) => item.label))).toEqual([
      '系统概览', '代码源', '代码扫描', 'API 资产', '场景发现', '待核验场景', '场景编排', '运行记录', '环境与密钥', '成员与权限',
    ])
    expect(groups[1].items[0].to).toBe('/systems/order%20center%2F%E5%8D%8E%E4%B8%9C/code-sources')
  })
})
