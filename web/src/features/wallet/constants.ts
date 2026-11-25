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
// ============================================================================
// Wallet Constants
// ============================================================================

/**
 * Default preset amount multipliers
 * Used to generate quick select amounts based on minimum topup
 */
export const DEFAULT_PRESET_MULTIPLIERS = [1, 5, 10, 30, 50, 100, 300, 500]

/**
 * Payment method types
 */
export const PAYMENT_TYPES = {
  ALIPAY: 'alipay',
  WECHAT: 'wxpay',
  STRIPE: 'stripe',
  CREEM: 'creem',
  WAFFO: 'waffo',
  WAFFO_PANCAKE: 'waffo_pancake',
} as const

/**
 * Default payment type
 */
export const DEFAULT_PAYMENT_TYPE = PAYMENT_TYPES.ALIPAY

/**
 * Payment icon colors (HEX format for react-icons)
 */
export const PAYMENT_ICON_COLORS = {
  [PAYMENT_TYPES.ALIPAY]: '#1677FF',
  [PAYMENT_TYPES.WECHAT]: '#07C160',
  [PAYMENT_TYPES.STRIPE]: '#635BFF',
  [PAYMENT_TYPES.CREEM]: '#6366F1',
  [PAYMENT_TYPES.WAFFO]: '#2563EB',
  [PAYMENT_TYPES.WAFFO_PANCAKE]: '#F97316',
} as const

/**
 * Default discount rate (no discount)
 */
export const DEFAULT_DISCOUNT_RATE = 1.0

/** Provider tag the backend puts on BEpusdt (USDT) payment methods */
export const BEPUSDT_PROVIDER = 'bepusdt'

/** How often the USDT payment dialog checks whether the order is paid */
export const BEPUSDT_STATUS_POLL_MS = 3000

/** Network names shown for BEpusdt trade types */
export const BEPUSDT_NETWORK_LABELS: Record<string, string> = {
  'usdt.trc20': 'TRC20 (Tron)',
  'usdt.erc20': 'ERC20 (Ethereum)',
  'usdt.bep20': 'BEP20 (BSC)',
  'usdt.polygon': 'Polygon',
  'usdt.arbitrum': 'Arbitrum One',
  'usdt.xlayer': 'X Layer',
  'usdt.plasma': 'Plasma',
  'usdt.solana': 'Solana',
}

/**
 * Default minimum topup amount
 */
export const DEFAULT_MIN_TOPUP = 1
