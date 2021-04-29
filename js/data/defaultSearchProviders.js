const defaultSearchProviders = [
  {
    id: 'mbz',
    name: 'Mobazha',
    logo: '../imgs/mbzSearchLogo.png',
    localLogo: '../imgs/mbzSearchLogo.png',
    listings: 'https://market.mobazha.com/api/search',
    torlistings: 'http://my7nrnmkscxr32zo.onion/listings/search',
    vendors: 'https://market.mobazha.com/api/search/vendors',
    torVendors: 'http://my7nrnmkscxr32zo.onion/profiles/search?type=vendor',
    reports: 'https://market.mobazha.com/search/reports',
    torReports: 'http://my7nrnmkscxr32zo.onion/reports',
  },
];

export default defaultSearchProviders;
