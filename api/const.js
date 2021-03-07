import { Platform } from 'react-native';

// let platformGatewayAPI;
// let platformWebsocketHost;

// if (Platform.OS === 'ios') {
//   platformGatewayAPI = 'http://45.32.240.156:4002';
//   platformWebsocketHost = '45.32.240.156:4002';
// } else {
//   platformGatewayAPI = 'http://localhost:4002';
//   platformWebsocketHost = 'localhost:4002';
// }

export const searchAPI = 'https://search.ob1.io';
export const stagingSearchAPI = 'https://staging.search.ob1.io';
export const obGatewayAPI = 'https://gateway2.ob1.io';
export const obEthGatewayAPI = 'https://gateway2.ob1.io';
export const gatewayAPI = 'http://localhost:4002';
export const websocketHost = 'localhost:4002';
// export const gatewayAPI = 'http://45.32.240.156:4002';
// export const websocketHost = '45.32.240.156:4002';
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
export const bestsellersListingAPI = 'https://search.ob1.io/listings/random?peerID=QmbdK6Tr5YNeQipYThphKc8iz3RWnuxfgQGb4VgyQsf7b5&ps=9';
export const gamingListingAPI = 'https://search.ob1.io/listings/random?peerID=QmQCoS3wvMH65CQ4TwpBxgYqdvNDmpY7DrVTMsfgzGcxHw&ps=9';
export const munchiesListingAPI = 'https://search.ob1.io/listings/random?peerID=QmX7Lg3tynbTGwSCZJT3AwAEgBoTg526TFEkYjRhKqPaN2&ps=9';
export const devicesListingAPI = 'https://search.ob1.io/listings/random?peerID=Qmb3bCU68g6za6wM3YkFMp1JhVJ8GAgzhdYgyrA3qHcRqd&ps=9';
export const trendListingAPI = 'https://raw.githubusercontent.com/OB1Company/HavenDiscover/master/trending.json';

// export const tokenServerAPI = 'https://getstreamauth.dev.ob1.io/';
export const streamTokenAPI = 'https://stream.mobazha.com/';
// export const streamTokenAPI = 'http://localhost:5001/';
// export const streamTokenAPI = 'http://192.168.0.169:5001/';
