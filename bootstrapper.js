// install babel hooks in the main process
if (process.env.NODE_ENV === 'development') {
  require('@babel/register'); // eslint-disable-line global-require
  require('./main'); // eslint-disable-line global-require
} else {
  require('./mainBuilt'); // eslint-disable-line global-require, import/no-unresolved
}
