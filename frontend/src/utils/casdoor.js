import Sdk from "casdoor-js-sdk";

const sdkConfig = {
  serverUrl: "https://login.mobazha.org",
  clientId: "44b16199e0b7b1d64b25",
  organizationName: "mobazha",
  appName: "app_mobazha",
  redirectPath: "/callback",
};
  
export const CasdoorSdk = new Sdk(sdkConfig);

export const hosting_server = 'https://store.mobazha.org';

export const isLoggedIn = () => {
  const token = localStorage.getItem("token");
  return token !== null && token.length > 0;
};

export const getSigninUrl = () => {
  return CasdoorSdk.getSigninUrl();
};

export function getSignupUrl() {
  return CasdoorSdk.getSignupUrl();
}

export function signin() {
  return CasdoorSdk.signin(hosting_server);
}

export const setToken = (token) => {
  localStorage.setItem("token", token);
};

export const logout = () => {
  localStorage.removeItem("token");
};

export const goToLink = (link) => {
  window.location.href = link;
};

export const getUserinfo = () => {
  return fetch(`${hosting_server}/api/userinfo`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${localStorage.getItem("token")}`,
    },
  }).then((res) => res.json());
};

export const getUsers = () => {
  return fetch(`${sdkConfig.serverUrl}/api/get-users?owner=${sdkConfig.organizationName}`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${localStorage.getItem("token")}`,
    },
  }).then((res) => res.json());
};
