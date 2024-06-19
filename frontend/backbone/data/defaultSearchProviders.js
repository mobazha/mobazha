import app from '../app';

const defaultSearchProviders = [
  import.meta.env.VITE_APP ? {
    id: 'mbz',
    name: 'Mobazha',
    logo: '../imgs/mbzSearchLogo.png',
    localLogo: '../imgs/mbzSearchLogo.png',
    listings: `https://${app.serverConfig.testnet ? 'console.' : ''}mobazha.info`,
    torlistings: 'http://my7nrnmkscxr32zo.onion/listings/search',
    vendors: `https://${app.serverConfig.testnet ? 'console.' : ''}mobazha.info/api/profile`,
    torVendors: 'http://my7nrnmkscxr32zo.onion/profiles/search?type=vendor',
    reports: `https://${app.serverConfig.testnet ? 'console.' : ''}mobazha.info/api/reports`,
    torReports: 'http://my7nrnmkscxr32zo.onion/reports',
  } : {
    // browser mode
    id: 'mbz',
    name: 'Mobazha',
    logo: '../imgs/mbzSearchLogo.png',
    localLogo: '../imgs/mbzSearchLogo.png',
    listings: `${location.origin}/info`,
    torlistings: 'http://my7nrnmkscxr32zo.onion/listings/search',
    vendors: `${location.origin}/info/api/profile`,
    torVendors: 'http://my7nrnmkscxr32zo.onion/profiles/search?type=vendor',
    reports: `${location.origin}/info/api/reports`,
    torReports: 'http://my7nrnmkscxr32zo.onion/reports',
  },
];

export default defaultSearchProviders;
