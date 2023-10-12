/**
 * Based on the route arguments, determine whether we
 * have a valid user route.
 */
export function isValidUserRoute(guid, state, slug) {
  const userStates = ['home', 'store', 'following', 'followers', 'reputation'];

  if (!guid || userStates.indexOf(state) === -1) {
    return false;
  }

  // so far store is the only state that could have
  // route parts beyond the state, e.g @themes/store/<slug>
  if (state !== 'store' && slug) {
    return false;
  }

  return true;
}
