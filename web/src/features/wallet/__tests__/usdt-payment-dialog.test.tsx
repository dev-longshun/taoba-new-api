/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import { api } from '@/lib/api'

import { UsdtPaymentDialog } from '../components/dialogs/usdt-payment-dialog'
import type { BEpusdtOrderPayload } from '../types'

const i18n = createInstance()

const order: BEpusdtOrderPayload = {
  trade_no: 'USR1NOabc1231700000000',
  trade_id: 'tid1',
  trade_type: 'usdt.polygon',
  address: '0xd75da239715e3025bdebb294cff7a39141a41777',
  amount: '10.27',
  money: '73.00',
  expire_in: 1800,
}

beforeEach(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: false,
    nsSeparator: false,
    resources: { en },
    interpolation: { escapeValue: false },
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})

function renderDialog(
  props: Partial<Parameters<typeof UsdtPaymentDialog>[0]> = {}
) {
  const onPaid = vi.fn()
  const onOpenChange = vi.fn()
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <UsdtPaymentDialog
          order={order}
          onPaid={onPaid}
          onOpenChange={onOpenChange}
          {...props}
        />
      </I18nextProvider>
    </QueryClientProvider>
  )
  return { onPaid, onOpenChange }
}

it('shows the network, exact amount and address of an open order', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { message: 'success', data: { status: 'pending' } },
  })

  renderDialog()

  expect(await screen.findByText('10.27')).toBeVisible()
  expect(screen.getByText(order.address)).toBeVisible()
  expect(screen.getByText('Network: Polygon')).toBeVisible()
  expect(
    screen.getByRole('button', { name: 'Amount (USDT)' })
  ).toBeInTheDocument()
})

it('reports a confirmed payment once and closes the dialog', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { message: 'success', data: { status: 'success' } },
  })

  const { onPaid, onOpenChange } = renderDialog()

  await waitFor(() => expect(onPaid).toHaveBeenCalledTimes(1))
  expect(onOpenChange).toHaveBeenCalledWith(false)
  expect(api.get).toHaveBeenCalledWith(
    '/api/user/bepusdt/order_status',
    expect.objectContaining({ params: { trade_no: order.trade_no } })
  )
})

it('shows the expired notice and stops checking an expired order', async () => {
  const get = vi.spyOn(api, 'get')

  renderDialog({ order: { ...order, expire_in: 0 } })

  expect(
    await screen.findByText(
      'This order has expired. Do not transfer to this address.'
    )
  ).toBeVisible()
  expect(screen.queryByText('Waiting for payment...')).not.toBeInTheDocument()
  expect(get).not.toHaveBeenCalled()
})
