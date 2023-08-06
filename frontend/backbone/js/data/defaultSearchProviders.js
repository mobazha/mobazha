import app from '../app';

const defaultSearchProviders = [
  {
    id: 'mbz',
    name: 'Mobazha',
    logo: '../../imgs/mbzSearchLogo.png',
    localLogo: '../../imgs/mbzSearchLogo.png',
    listings: `https://${app.serverConfig.testnet ? 'console.' : ''}mobazha.info`,
    torlistings: 'http://my7nrnmkscxr32zo.onion/listings/search',
    vendors: `https://${app.serverConfig.testnet ? 'console.' : ''}mobazha.info/api/profile`,
    torVendors: 'http://my7nrnmkscxr32zo.onion/profiles/search?type=vendor',
    reports: `https://${app.serverConfig.testnet ? 'console.' : ''}mobazha.info/api/reports`,
    torReports: 'http://my7nrnmkscxr32zo.onion/reports',
  },
];

export default defaultSearchProviders;
