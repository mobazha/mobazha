export function checkValidParticipantObject(participant, type) {
  if (typeof participant !== 'object') {
    throw new Error(`Please provide a participant object for the ${type}.`);
  }

  if (typeof type !== 'string') {
    throw new Error('Please provide the participant type as a string.');
  }

  if (!participant.id || typeof participant.getProfile !== 'function') {
    throw new Error(
      `The ${type} object is not valid. It should have an id ` +
        'as well as a getProfile function that returns a promise that ' +
        'resolves with a profile model.',
    );
  }
}
