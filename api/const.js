import { Platform } from 'react-native';

// let platformGatewayAPI;
// let platformWebsocketHost;

// if (Platform.OS === 'ios') {
//   platformGatewayAPI = 'https://gateway.mobazha.com';
//   platformWebsocketHost = 'gateway.mobazha.com';
// } else {
//   platformGatewayAPI = 'http://localhost:4002';
//   platformWebsocketHost = 'localhost:4002';
// }

export const searchAPI = 'https://mobazha.com.info/api';
export const stagingSearchAPI = 'https://staging.search.ob1.io';
export const mbzGatewayAPI = 'https://gateway.mobazha.com';
export const mbzEthGatewayAPI = 'https://gateway.mobazha.com';
export const gatewayAPI = 'http://localhost:4002';
export const websocketHost = 'localhost:4002';
// export const gatewayAPI = 'http://gateway.mobazha.com';
// export const websocketHost = 'gateway.mobazha.com';
// export const gatewayAPI = 'http://45.76.186.204:4002'; // moderator node
// export const websocketHost = '45.76.186.204:4002'; // moderator node
// export const gatewayAPI = platformGatewayAPI; // seller node
// export const websocketHost = platformWebsocketHost; // seller node
// export const gatewayAPI = 'http://149.28.177.216:4002'; // buyer node
// export const websocketHost = '149.28.177.216:4002'; // buyer node
export const featuredAPI =
  'https://gist.githubusercontent.com/drwasho/32036329b2a04e0262dae0191f873f35/raw/featuredStores.json';
export const promoAPI =
  'https://raw.githubusercontent.com/OB1Company/HavenDiscover/master/discover2.json';
export const featuredListingAPI = 'https://raw.githubusercontent.com/OB1Company/HavenDiscover/master/featuredListings.json';
export const bestsellersListingAPI = 'https://mobazha.com.info/api/profile/listings?peerID=QmU6X61YTJT3v3VpkMSeeoU3d9Cik9EQJNRWyTgx8qe5r6&pageSize=9';
export const gamingListingAPI = 'https://mobazha.com.info/api/profile/listings?peerID=QmQCoS3wvMH65CQ4TwpBxgYqdvNDmpY7DrVTMsfgzGcxHw&pageSize=9';
export const munchiesListingAPI = 'https://mobazha.com.info/api/profile/listings?peerID=QmX7Lg3tynbTGwSCZJT3AwAEgBoTg526TFEkYjRhKqPaN2&pageSize=9';
export const devicesListingAPI = 'https://mobazha.com.info/api/profile/listings?peerID=Qmb3bCU68g6za6wM3YkFMp1JhVJ8GAgzhdYgyrA3qHcRqd&pageSize=9';
export const trendListingAPI = 'https://raw.githubusercontent.com/OB1Company/HavenDiscover/master/trending.json';

// export const tokenServerAPI = 'https://getstreamauth.dev.ob1.io/';
export const streamTokenAPI = 'https://stream.mobazha.info/';
// export const streamTokenAPI = 'http://localhost:5001/';
// export const streamTokenAPI = 'http://192.168.0.169:5001/';
