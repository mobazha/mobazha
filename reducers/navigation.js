import { NavigationActions } from 'react-navigation';
import AppNavigator from '../routes';

import {I18n} from '../langs/I18n';

const action = AppNavigator.router.getActionForPathAndParams(null);
const initialState = AppNavigator.router.getStateForAction({});
// const initialState = {};
export const actions = {
  navigate: 'CUSTOM_NAV/NAVIGATE',
};

const navReducer = (state = initialState, action) => {
  let nextState;
  switch (action.type) {
    case actions.navigate:
      nextState = AppNavigator.router.getStateForAction(
        NavigationActions.navigate({
          routeName: action.payload,
        }),
        state,
      );
      break;
    default:
      nextState = AppNavigator.router.getStateForAction(action, state);
      break;
  }

  // Return original `state` if `nextState` is null/undefined
  return nextState || state;
};

export default navReducer;
