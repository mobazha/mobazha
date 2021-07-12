import {I18n} from '../langs/I18n';

const FEE_PLANS = [
  {
    label: I18n.t('config.feePlans.Super_economic_v'),
    value: 'SUPER_ECONOMIC',
    displayLabel: I18n.t('config.feePlans.Super_Economic'),
  },
  {
    label: I18n.t('config.feePlans.Economic_v'),
    value: 'ECONOMIC',
    displayLabel: I18n.t('config.feePlans.Economic'),
  },
  {
    label: I18n.t('config.feePlans.Normal_v'),
    value: 'NORMAL',
    displayLabel: I18n.t('config.feePlans.Normal'),
  },
  {
    label: I18n.t('config.feePlans.Priority_v'),
    value: 'PRIORITY',
    displayLabel: I18n.t('config.feePlans.Priority'),
  },
];

export default FEE_PLANS;
