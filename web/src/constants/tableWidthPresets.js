export const CHANNEL_DETAIL_MODEL_COLUMN_WIDTHS = {
  selection: 64,
  name: '20%',
  type: 72,
  alias: '22%',
  priceUnit: 108,
  price: '16%',
  status: 92,
  upstreamReturn: 140,
  actions: 176,
};

export const PROVIDER_LIST_COLUMN_WIDTHS = {
  id: '22%',
  name: '28%',
  createdAt: 148,
  updatedAt: 148,
  actions: 72,
};

export const TOPUP_PLAN_LIST_COLUMN_WIDTHS = {
  name: '16%',
  group: '18%',
  payAmount: '13%',
  creditedAmount: '13%',
  sortOrder: 100,
  enabled: 100,
  publicVisible: 100,
  validityDays: 100,
  actions: 176,
};

export const PACKAGE_LIST_COLUMN_WIDTHS = {
  name: 120,
  group: 110,
  salePrice: 80,
  dailyQuota: 120,
  emergencyQuota: 120,
  durationDays: 80,
  status: 92,
  createdAt: 148,
  updatedAt: 148,
  actions: 176,
};

export const PACKAGE_LIST_TABLE_MIN_WIDTH =
  PACKAGE_LIST_COLUMN_WIDTHS.name +
  PACKAGE_LIST_COLUMN_WIDTHS.group +
  PACKAGE_LIST_COLUMN_WIDTHS.salePrice +
  PACKAGE_LIST_COLUMN_WIDTHS.dailyQuota +
  PACKAGE_LIST_COLUMN_WIDTHS.emergencyQuota +
  PACKAGE_LIST_COLUMN_WIDTHS.durationDays +
  PACKAGE_LIST_COLUMN_WIDTHS.status +
  PACKAGE_LIST_COLUMN_WIDTHS.createdAt +
  PACKAGE_LIST_COLUMN_WIDTHS.updatedAt +
  PACKAGE_LIST_COLUMN_WIDTHS.actions;

export const CHANNEL_LIST_COLUMN_WIDTHS = {
  selection: 48,
  name: '16%',
  type: 88,
  status: 92,
  createdAt: 148,
  updatedAt: 148,
  capabilities: '14%',
  balance: '12%',
  priority: 92,
  actions: 192,
};

export const GROUP_LIST_COLUMN_WIDTHS = {
  name: '14%',
  description: '20%',
  channels: '20%',
  billingRatio: 96,
  status: 92,
  createdAt: 148,
  updatedAt: 148,
  actions: 192,
};

export const TOKEN_LIST_COLUMN_WIDTHS = {
  name: '14%',
  status: 92,
  token: '20%',
  usedAmount: '12%',
  remainingAmount: '12%',
  createdTime: 148,
  expiredTime: 148,
  actions: 228,
};

export const USER_LIST_COLUMN_WIDTHS = {
  username: '16%',
  wallet: '14%',
  package: '14%',
  balance: '12%',
  requestCount: 100,
  createdAt: 148,
  updatedAt: 148,
  role: 92,
  status: 92,
  actions: 176,
};

export const REDEMPTION_LIST_COLUMN_WIDTHS = {
  name: '14%',
  group: '14%',
  status: 92,
  faceValue: '14%',
  createdTime: 148,
  codeExpiresAt: 148,
  redeemedTime: 148,
  actions: 192,
};

export const LOG_LIST_COLUMN_WIDTHS = {
  time: 148,
  channel: '12%',
  group: '12%',
  type: 88,
  model: '14%',
  username: '12%',
  tokenName: '12%',
  promptTokens: 96,
  completionTokens: 96,
  quota: '12%',
};

export const BUSINESS_FLOW_COLUMN_WIDTHS = {
  user: '12%',
  userCompact: '10%',
  status: 92,
  type: 88,
  source: '16%',
  amount: '12%',
  quota: '12%',
  datetime: 148,
  group: '12%',
  packageName: '14%',
  model: '14%',
  order: '18%',
  message: 180,
  tokenCount: 96,
  actions: 120,
};

export const TASK_LIST_COLUMN_WIDTHS = {
  type: 92,
  user: '12%',
  channel: '12%',
  model: '16%',
  status: 92,
  createdAt: 148,
  updatedAt: 148,
  actionsCompact: 88,
  actionsWide: 228,
};

export const BALANCE_LOT_COLUMN_WIDTHS = {
  source: '18%',
  remaining: '16%',
  total: '16%',
  status: 92,
  grantedAt: 148,
  expiresAt: 148,
};

export const TOPUP_RESULT_COLUMN_WIDTHS = {
  label: '20%',
  value: '30%',
};

export const TOPUP_RECORD_COLUMN_WIDTHS = {
  time: 148,
  businessType: 92,
  status: 92,
  amount: '14%',
  quotaOrPackage: '16%',
  redemptionCode: '24%',
  actions: 228,
};
