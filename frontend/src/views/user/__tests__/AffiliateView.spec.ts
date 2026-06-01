import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const affiliateDetailState = vi.hoisted(() => ({
  value: {
    user_id: 7,
    aff_code: 'AFF123',
    inviter_id: null,
    aff_count: 3,
    aff_quota: 88,
    aff_frozen_quota: 0,
    aff_history_quota: 120,
    effective_rebate_rate_percent: 20,
    is_distributor_enabled: false,
    invitees: [],
  },
}))

const getAffiliateDetail = vi.hoisted(() => vi.fn())
const transferAffiliateQuota = vi.hoisted(() => vi.fn())
const refreshUser = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const copyToClipboard = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/api/user', () => ({
  __esModule: true,
  default: {
    getAffiliateDetail,
    transferAffiliateQuota,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

import AffiliateView from '../AffiliateView.vue'

describe('AffiliateView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getAffiliateDetail.mockReset()
    transferAffiliateQuota.mockReset()
    refreshUser.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    copyToClipboard.mockReset()
    affiliateDetailState.value = {
      user_id: 7,
      aff_code: 'AFF123',
      inviter_id: null,
      aff_count: 3,
      aff_quota: 88,
      aff_frozen_quota: 0,
      aff_history_quota: 120,
      effective_rebate_rate_percent: 20,
      is_distributor_enabled: false,
      invitees: [],
    }
    getAffiliateDetail.mockResolvedValue(affiliateDetailState.value)
  })

  it('disables transfer for distributor-enabled users and shows the distributor hint', async () => {
    affiliateDetailState.value = {
      ...affiliateDetailState.value,
      is_distributor_enabled: true,
    }
    getAffiliateDetail.mockResolvedValue(affiliateDetailState.value)

    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: { template: '<span />' },
        },
      },
    })

    await flushPromises()

    const transferButton = wrapper.find('button.btn.btn-primary')
    expect(transferButton.exists()).toBe(true)
    expect(transferButton.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('affiliate.transfer.distributorDisabled')

    await transferButton.trigger('click')

    expect(transferAffiliateQuota).not.toHaveBeenCalled()
  })

  it('keeps transfer available for regular affiliate users', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: { template: '<span />' },
        },
      },
    })

    await flushPromises()

    const transferButton = wrapper.find('button.btn.btn-primary')
    expect(transferButton.attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).not.toContain('affiliate.transfer.distributorDisabled')
  })
})
