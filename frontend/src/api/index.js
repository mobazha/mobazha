import { ipcApiRoute } from "./main"
import chat from "./chat";
import shoppingCart from "./shoppingCart";
export default {
  ipcApiRoute,
  ...chat,
  ...shoppingCart,
};
